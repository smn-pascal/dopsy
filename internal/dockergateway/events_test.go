package dockergateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/smn-pascal/dopsy/internal/domain"
)

func eventQuery(id string, since, until int64) url.Values {
	filters, _ := json.Marshal(map[string][]string{"container": {id}, "type": {"container"}, "event": eventFilters})
	return url.Values{"since": {strconv.FormatInt(since, 10)}, "until": {strconv.FormatInt(until, 10)}, "filters": {string(filters)}}
}

func TestEventAllowlistRequiresBoundedScopedPastWindow(t *testing.T) {
	t.Parallel()
	const now = int64(200000)
	tests := []struct {
		name    string
		change  func(url.Values)
		allowed bool
	}{
		{"valid", func(url.Values) {}, true},
		{"24 hours", func(q url.Values) { q.Set("since", "113599"); q.Set("until", "199999") }, true},
		{"no start", func(q url.Values) { q.Del("since") }, false},
		{"no end", func(q url.Values) { q.Del("until") }, false},
		{"zero start", func(q url.Values) { q.Set("since", "0") }, false},
		{"future end", func(q url.Values) { q.Set("until", "200001") }, false},
		{"current end", func(q url.Values) { q.Set("until", "200000") }, false},
		{"same endpoints", func(q url.Values) { q.Set("until", "199900") }, false},
		{"reversed", func(q url.Values) { q.Set("until", "199000") }, false},
		{"over 24 hours", func(q url.Values) { q.Set("since", "113598") }, false},
		{"duplicate timestamp", func(q url.Values) { q.Add("since", "100") }, false},
		{"timestamp overflow", func(q url.Values) { q.Set("until", "999999999999999999999") }, false},
		{"unknown query", func(q url.Values) { q.Set("stream", "true") }, false},
		{"no filters", func(q url.Values) { q.Del("filters") }, false},
		{"broad filter", func(q url.Values) { q.Set("filters", `{}`) }, false},
		{"malformed filter", func(q url.Values) { q.Set("filters", `{"container":`) }, false},
		{"multiple containers", func(q url.Values) {
			q.Set("filters", `{"container":["a","b"],"type":["container"],"event":["start","stop","die","oom","restart","kill","health_status"]}`)
		}, false},
		{"label filter", func(q url.Values) {
			q.Set("filters", `{"container":["abc"],"type":["container"],"label":["secret"],"event":["start","stop","die","oom","restart","kill","health_status"]}`)
		}, false},
		{"exec action", func(q url.Values) {
			q.Set("filters", `{"container":["abc"],"type":["container"],"event":["start","stop","die","oom","restart","kill","exec_start"]}`)
		}, false},
		{"duplicate actions", func(q url.Values) {
			q.Set("filters", `{"container":["abc"],"type":["container"],"event":["start","stop","die","oom","restart","kill","kill"]}`)
		}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := eventQuery("abc", 199900, 199999)
			test.change(q)
			if got := validContainerEventsQuery(q, now); got != test.allowed {
				t.Fatalf("allowed=%v want=%v", got, test.allowed)
			}
		})
	}
	query := eventQuery("abc", time.Now().Add(-time.Hour).Unix(), time.Now().Add(-time.Second).Unix())
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodDelete} {
		request := httptest.NewRequest(method, "http://docker/v1.47/events?"+query.Encode(), nil)
		if got := AllowedDockerRequest(request); got != (method == http.MethodGet || method == http.MethodHead) {
			t.Fatalf("%s allowed=%v", method, got)
		}
	}
}

func TestEventProxySanitizesAndRechecksScope(t *testing.T) {
	t.Parallel()
	query := eventQuery("abc", 100, 200)
	body := `{"Type":"container","Action":"oom","Actor":{"ID":"abc","Attributes":{"password":"hunter2","name":"private","command":"curl secret"}},"time":150,"from":"secret-image","status":"private"}` + "\n" +
		`{"Type":"container","Action":"exec_start: secret command","Actor":{"ID":"abc"},"time":151}` + "\n" +
		`{"Type":"container","Action":"die","Actor":{"ID":"other"},"time":152}` + "\n" +
		`{"Type":"container","Action":"start","Actor":{"ID":"abc"},"time":99}` + "\n" +
		`{"Type":"network","Action":"start","Actor":{"ID":"abc"},"time":153}`
	proxy := NewReadOnlyProxyWithTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{eventsTruncatedHeader: {"true"}}, Request: r}, nil
	}))
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, httptest.NewRequest("GET", "http://docker/events?"+query.Encode(), nil))
	if response.Code != 200 {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	for _, forbidden := range []string{"hunter2", "Attributes", "private", "command", "other", "network", "exec_start"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("leaked %q: %s", forbidden, response.Body.String())
		}
	}
	if response.Header().Get(eventsTruncatedHeader) != "" {
		t.Fatal("upstream truncation header was trusted")
	}
	var event rawEvent
	if json.Unmarshal(response.Body.Bytes(), &event) != nil || event.Action != "oom" || event.Time != 150 {
		t.Fatalf("event=%+v", event)
	}
}

func TestEventReadBoundsAndMalformedResponse(t *testing.T) {
	t.Parallel()
	options := domain.EventOptions{Since: 100, Until: 300}
	record := `{"Type":"container","Action":"die","Actor":{"ID":"abc"},"time":150}` + "\n"
	events, truncated, err := readEvents(strings.NewReader(strings.Repeat(record, MaxEvents+1)), "abc", options)
	if err != nil || !truncated || len(events) != MaxEvents {
		t.Fatalf("count=%d truncated=%v err=%v", len(events), truncated, err)
	}
	if _, _, err := readEvents(strings.NewReader(strings.Repeat("x", MaxEventBytes+1)), "abc", options); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("error=%v", err)
	}
	if _, _, err := readEvents(strings.NewReader(record+`{"Type":`), "abc", options); err == nil {
		t.Fatal("malformed response accepted")
	}
	empty, truncated, err := readEvents(strings.NewReader(""), "abc", options)
	if err != nil || truncated || empty == nil || len(empty) != 0 {
		t.Fatalf("empty=%v err=%v", empty, err)
	}
}

func TestEngineEventsThroughProxyAndNameResolution(t *testing.T) {
	t.Parallel()
	now := time.Now().Unix()
	options := domain.EventOptions{Since: now - 3600, Until: now - 1}
	reads := 0
	proxy := httptest.NewServer(NewReadOnlyProxyWithTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		reads++
		body := `{"Id":"resolved-id"}`
		if r.URL.Path == "/events" {
			id, ok := decodeEventFilters(r.URL.Query().Get("filters"))
			if !ok || id != "resolved-id" {
				t.Errorf("filter=%s", r.URL.RawQuery)
			}
			body = fmt.Sprintf(`{"Type":"container","Action":"oom","Actor":{"ID":"resolved-id","Attributes":{"TOKEN":"secret"}},"time":%d}`, now-100)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
	})))
	defer proxy.Close()
	engine, err := NewEngine(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	events, err := engine.ContainerEvents(context.Background(), "api-name", options)
	if err != nil {
		t.Fatal(err)
	}
	if reads != 2 || len(events.Items) != 1 || events.Items[0].Action != "oom" || !events.HistoryLimited || events.Truncated || events.Since != options.Since {
		t.Fatalf("reads=%d events=%+v", reads, events)
	}
	for _, invalid := range []domain.EventOptions{{}, {Since: now - 10, Until: now + 100}, {Since: now - 90000, Until: now - 1}} {
		if _, err := engine.ContainerEvents(context.Background(), "api-name", invalid); err == nil {
			t.Fatal("invalid window accepted")
		}
	}
	if reads != 2 {
		t.Fatal("invalid window reached Docker")
	}
}

func TestDemoEventsRespectRequestedWindow(t *testing.T) {
	t.Parallel()
	now := time.Unix(200000, 0)
	demo := NewDemo()
	demo.now = func() time.Time { return now }
	events, err := demo.ContainerEvents(context.Background(), "demo-api", domain.EventOptions{Since: now.Unix() - 1000, Until: now.Unix() - 1})
	if err != nil || len(events.Items) != 3 || events.Items[0].Action != "oom" || !events.HistoryLimited {
		t.Fatalf("events=%+v err=%v", events, err)
	}
	empty, err := demo.ContainerEvents(context.Background(), "demo-api", domain.EventOptions{Since: now.Unix() - 50, Until: now.Unix() - 1})
	if err != nil || len(empty.Items) != 0 || !empty.HistoryLimited {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}
}

func TestProxyEventTruncationIsPropagatedToEngine(t *testing.T) {
	t.Parallel()
	now := time.Now().Unix()
	options := domain.EventOptions{Since: now - 3600, Until: now - 1}
	proxy := httptest.NewServer(NewReadOnlyProxyWithTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if deadline, ok := r.Context().Deadline(); r.URL.Path == "/events" && (!ok || time.Until(deadline) > 12*time.Second) {
			t.Error("proxy event deadline missing")
		}
		body := `{"Id":"abc"}`
		if r.URL.Path == "/events" {
			body = strings.Repeat(fmt.Sprintf(`{"Type":"container","Action":"die","Actor":{"ID":"abc"},"time":%d}`+"\n", now-20), MaxEvents+1)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
	})))
	defer proxy.Close()
	engine, _ := NewEngine(proxy.URL)
	result, err := engine.ContainerEvents(context.Background(), "abc", options)
	if err != nil || !result.Truncated || !result.HistoryLimited || len(result.Items) != MaxEvents {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestProxyBlocksUnsafeEventRequestBeforeUpstream(t *testing.T) {
	t.Parallel()
	called := false
	proxy := NewReadOnlyProxyWithTransport(roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("should not be reached")
	}))
	for _, target := range []string{"http://docker/events", "http://docker/events?since=1&until=100&filters=%7B%7D"} {
		response := httptest.NewRecorder()
		proxy.ServeHTTP(response, httptest.NewRequest("GET", target, nil))
		if response.Code != http.StatusForbidden || called {
			t.Fatalf("status=%d called=%v", response.Code, called)
		}
	}
}

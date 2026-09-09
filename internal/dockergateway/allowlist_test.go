package dockergateway

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/smn-pascal/dopsy/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestAllowedDockerRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		method  string
		target  string
		allowed bool
	}{
		{name: "ping", method: http.MethodGet, target: "http://docker/_ping", allowed: true},
		{name: "version", method: http.MethodHead, target: "http://docker/version", allowed: true},
		{name: "versioned list", method: http.MethodGet, target: "http://docker/v1.47/containers/json?all=1", allowed: true},
		{name: "list true", method: http.MethodGet, target: "http://docker/containers/json?all=true", allowed: true},
		{name: "inspect", method: http.MethodGet, target: "http://docker/containers/abc123/json", allowed: true},
		{name: "minimal bounded logs", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=1", allowed: true},
		{name: "bounded logs", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=2000&stdout=1&stderr=true&timestamps=1&since=100&until=86500", allowed: true},
		{name: "non-streaming stats", method: http.MethodGet, target: "http://docker/containers/abc123/stats?stream=false", allowed: true},
		{name: "non-streaming stats zero", method: http.MethodGet, target: "http://docker/containers/abc123/stats?stream=0", allowed: true},
		{name: "events removed", method: http.MethodGet, target: "http://docker/events", allowed: false},
		{name: "post list", method: http.MethodPost, target: "http://docker/containers/json", allowed: false},
		{name: "delete", method: http.MethodDelete, target: "http://docker/containers/abc123", allowed: false},
		{name: "archive", method: http.MethodGet, target: "http://docker/containers/abc123/archive", allowed: false},
		{name: "export", method: http.MethodGet, target: "http://docker/containers/abc123/export", allowed: false},
		{name: "top", method: http.MethodGet, target: "http://docker/containers/abc123/top", allowed: false},
		{name: "images", method: http.MethodGet, target: "http://docker/images/json", allowed: false},
		{name: "exec", method: http.MethodGet, target: "http://docker/exec/abc123/json", allowed: false},
		{name: "encoded slash", method: http.MethodGet, target: "http://docker/containers/abc123%2Fexport/json", allowed: false},
		{name: "ping query", method: http.MethodGet, target: "http://docker/_ping?verbose=1", allowed: false},
		{name: "list missing all", method: http.MethodGet, target: "http://docker/containers/json", allowed: false},
		{name: "list all false", method: http.MethodGet, target: "http://docker/containers/json?all=0", allowed: false},
		{name: "list filters", method: http.MethodGet, target: "http://docker/containers/json?all=1&filters=%7B%7D", allowed: false},
		{name: "list duplicate all", method: http.MethodGet, target: "http://docker/containers/json?all=1&all=true", allowed: false},
		{name: "inspect query", method: http.MethodGet, target: "http://docker/containers/abc123/json?size=1", allowed: false},
		{name: "logs missing tail", method: http.MethodGet, target: "http://docker/containers/abc123/logs?stdout=1", allowed: false},
		{name: "logs zero tail", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=0", allowed: false},
		{name: "logs excessive tail", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=2001", allowed: false},
		{name: "logs non-integer tail", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=all", allowed: false},
		{name: "logs duplicate tail", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=1&tail=2", allowed: false},
		{name: "log following true", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=10&follow=true", allowed: false},
		{name: "log following false still rejected", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=10&follow=0", allowed: false},
		{name: "log details", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=10&details=0", allowed: false},
		{name: "log timestamps false", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=10&timestamps=0", allowed: false},
		{name: "log timestamps true word", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=10&timestamps=true", allowed: false},
		{name: "log unsafe bool", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=10&stdout=yes", allowed: false},
		{name: "log non-integer since", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=10&since=yesterday", allowed: false},
		{name: "log until only", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=10&until=1000", allowed: false},
		{name: "log reversed range", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=10&since=200&until=100", allowed: false},
		{name: "log excessive range", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=10&since=1&until=86402", allowed: false},
		{name: "log unknown query", method: http.MethodGet, target: "http://docker/containers/abc123/logs?tail=10&format=json", allowed: false},
		{name: "stat streaming", method: http.MethodGet, target: "http://docker/containers/abc123/stats?stream=1", allowed: false},
		{name: "stats missing stream", method: http.MethodGet, target: "http://docker/containers/abc123/stats", allowed: false},
		{name: "stats unknown query", method: http.MethodGet, target: "http://docker/containers/abc123/stats?stream=0&one-shot=1", allowed: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request, err := http.NewRequest(test.method, test.target, nil)
			if err != nil {
				t.Fatal(err)
			}
			if result := AllowedDockerRequest(request); result != test.allowed {
				t.Fatalf("AllowedDockerRequest() = %v, want %v", result, test.allowed)
			}
		})
	}
}

func TestLogQuerySinceOnlyIsLimitedToLastDay(t *testing.T) {
	t.Parallel()
	const now = int64(2_000_000)
	if !validContainerLogsQuery(url.Values{"tail": {"10"}, "since": {"1913600"}}, now) {
		t.Fatal("exactly 24 hour since-only window should be allowed")
	}
	if validContainerLogsQuery(url.Values{"tail": {"10"}, "since": {"1913599"}}, now) {
		t.Fatal("since-only window over 24 hours should be rejected")
	}
	if validContainerLogsQuery(url.Values{"tail": {"10"}, "since": {"2000001"}}, now) {
		t.Fatal("future since-only timestamp should be rejected")
	}
}

func TestAllowlistTransportBlocksBeforeUpstream(t *testing.T) {
	t.Parallel()
	called := false
	transport := AllowlistTransport{Next: roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, nil
	})}
	request, err := http.NewRequest(http.MethodPost, "http://docker/containers/abc/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transport.RoundTrip(request); err != ErrBlockedRequest {
		t.Fatalf("RoundTrip() error = %v, want %v", err, ErrBlockedRequest)
	}
	if called {
		t.Fatal("blocked request reached upstream transport")
	}
}

func TestReadOnlyProxyBlocksMutation(t *testing.T) {
	t.Parallel()
	called := false
	handler := NewReadOnlyProxyWithTransport(roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, nil
	}))
	request := httptest.NewRequest(http.MethodPost, "/containers/abc123/start", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if called {
		t.Fatal("blocked proxy request reached Docker")
	}
}

func TestReadOnlyProxySanitizesInspectResponse(t *testing.T) {
	t.Parallel()
	raw := `{
		"Id":"abc123","Name":"/api","Path":"/bin/server","Args":["--token","super-secret"],
		"RestartCount":4,
		"Config":{"Image":"example/api:1","Env":["PASSWORD=hunter2"],"Cmd":["server","--secret"],"Entrypoint":["/private/start"],"Labels":{"api.token":"secret-label","com.example.safe":"visible"}},
		"State":{"Status":"exited","Running":false,"OOMKilled":true,"ExitCode":137,"Error":"/host/private/error","StartedAt":"start","FinishedAt":"finish","Health":{"Status":"unhealthy","Log":[{"Output":"secret health output"}]}},
		"HostConfig":{"Memory":268435456},
		"Mounts":[{"Source":"/Users/private/database","Destination":"/data"}],
		"LogPath":"/var/lib/docker/private.log"
	}`
	handler := NewReadOnlyProxyWithTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(raw)),
			Request:    request,
		}, nil
	}))
	request := httptest.NewRequest(http.MethodGet, "/containers/abc123/json", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	for _, forbidden := range []string{"hunter2", "super-secret", "secret-label", "/Users/private", "/var/lib/docker", "secret health output", "Entrypoint", "Mounts", "Args", "Path"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Errorf("sanitized response contains %q: %s", forbidden, response.Body.String())
		}
	}
	var sanitized rawInspection
	if err := json.Unmarshal(response.Body.Bytes(), &sanitized); err != nil {
		t.Fatal(err)
	}
	inspection := sanitizeInspect(sanitized)
	if !inspection.OOMKilled || inspection.ExitCode != 137 || inspection.MemoryLimit != 268435456 || inspection.Image != "example/api:1" {
		t.Fatalf("required diagnostic fields were not preserved: %+v", inspection)
	}
}

func TestReadOnlyProxySanitizesContainerListResponse(t *testing.T) {
	t.Parallel()
	raw := `[{
		"Id":"abc123","Names":["/api","/secret-alias"],"Image":"example/api:1",
		"State":"running","Status":"Up 1 hour (healthy)","Created":1234,
		"Labels":{"api.token":"secret-label"},
		"Mounts":[{"Source":"/Users/private/database","Destination":"/data"}],
		"HostConfig":{"NetworkMode":"secret-network"},"Command":"server --token hunter2"
	}]`
	handler := NewReadOnlyProxyWithTransport(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(raw)),
			Request:    request,
		}, nil
	}))
	request := httptest.NewRequest(http.MethodGet, "/containers/json?all=1", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	for _, forbidden := range []string{"secret-alias", "secret-label", "/Users/private", "secret-network", "hunter2", "Labels", "Mounts", "HostConfig", "Command"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Errorf("sanitized list contains %q: %s", forbidden, response.Body.String())
		}
	}
	var containers []map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &containers); err != nil {
		t.Fatal(err)
	}
	if len(containers) != 1 || len(containers[0]) != 6 {
		t.Fatalf("unexpected safe container shape: %#v", containers)
	}
	names, ok := containers[0]["Names"].([]any)
	if !ok || len(names) != 1 || names[0] != "/api" {
		t.Fatalf("only the first name should remain: %#v", containers[0]["Names"])
	}
}

func TestSanitizeInspectDropsSensitiveFields(t *testing.T) {
	t.Parallel()
	var raw rawInspection
	input := `{"Id":"abc","Name":"/api","Path":"/bin/private","Args":["--password=x"],"Config":{"Image":"api:1","Env":["TOKEN=x"],"Cmd":["secret"],"Entrypoint":["private"],"Labels":{"secret":"x"}},"State":{"Status":"running","Running":true,"Health":{"Status":"healthy","Log":[{"Output":"token=x"}]}},"HostConfig":{"Memory":1024},"Mounts":[{"Source":"/host/private","Destination":"/data"}]}`
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatal(err)
	}
	sanitized := sanitizeInspect(raw)
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"TOKEN=x", "--password", "/host/private", "/bin/private", "token=x"} {
		if bytes.Contains(encoded, []byte(secret)) {
			t.Errorf("sanitized inspection contains %q: %s", secret, encoded)
		}
	}
	if sanitized.ID != "abc" || sanitized.Name != "api" || sanitized.Image != "api:1" || !sanitized.Running {
		t.Fatalf("safe fields missing: %+v", sanitized)
	}
}

func TestEngineUsesGETOnly(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", request.Method)
		}
		switch request.URL.Path {
		case "/_ping":
			_, _ = writer.Write([]byte("OK"))
		case "/containers/json":
			if request.URL.RawQuery != "all=1" {
				t.Errorf("list query = %q, want all=1", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`[]`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	engine, err := NewEngine(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	containers, err := engine.ListContainers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(containers) != 0 {
		t.Fatalf("containers = %v, want empty", containers)
	}
}

func TestEngineLogsUseSafeQueryAndDemultiplex(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/containers/abc123/logs" {
			http.NotFound(writer, request)
			return
		}
		query := request.URL.Query()
		if len(query) != 4 || query.Get("stdout") != "1" || query.Get("stderr") != "1" || query.Get("timestamps") != "1" || query.Get("tail") != "25" {
			t.Errorf("unsafe or incomplete log query: %s", request.URL.RawQuery)
		}
		if _, present := query["follow"]; present {
			t.Errorf("follow must not be sent: %s", request.URL.RawQuery)
		}
		_, _ = writer.Write(dockerLogFrame(1, []byte("2026-09-09T00:00:00Z hello\n")))
		_, _ = writer.Write(dockerLogFrame(2, []byte("2026-09-09T00:00:01Z warning\n")))
	}))
	defer server.Close()
	engine, err := NewEngine(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	logs, err := engine.ContainerLogs(context.Background(), "abc123", domain.LogOptions{Tail: 25})
	if err != nil {
		t.Fatal(err)
	}
	if logs.Truncated || logs.Text != "2026-09-09T00:00:00Z hello\n2026-09-09T00:00:01Z warning\n" {
		t.Fatalf("unexpected logs: %+v", logs)
	}
}

func TestReadDockerLogsTruncatesMultiplexedPayloadWithoutHeaders(t *testing.T) {
	t.Parallel()
	wire := append(dockerLogFrame(1, []byte("abc")), dockerLogFrame(2, []byte("defgh"))...)
	logs, truncated, err := readDockerLogs(bytes.NewReader(wire), 5)
	if err != nil {
		t.Fatal(err)
	}
	if !truncated || string(logs) != "abcde" {
		t.Fatalf("logs=%q truncated=%v, want %q true", logs, truncated, "abcde")
	}
	if bytes.Contains(logs, []byte{2, 0, 0, 0}) {
		t.Fatalf("multiplex header leaked into output: %v", logs)
	}
}

func TestReadDockerLogsHandlesRawTTYStream(t *testing.T) {
	t.Parallel()
	logs, truncated, err := readDockerLogs(strings.NewReader("plain tty output"), 9)
	if err != nil {
		t.Fatal(err)
	}
	if !truncated || string(logs) != "plain tty" {
		t.Fatalf("logs=%q truncated=%v, want %q true", logs, truncated, "plain tty")
	}
}

func TestReadDockerLogsRejectsIncompleteMultiplexFrame(t *testing.T) {
	t.Parallel()
	wire := dockerLogFrame(1, []byte("complete"))
	wire = append(wire, []byte{2, 0, 0, 0}...)
	if _, _, err := readDockerLogs(bytes.NewReader(wire), 1024); err == nil {
		t.Fatal("incomplete multiplex header should fail instead of leaking binary bytes")
	}
}

func TestSanitizeStatsSubtractsCacheAndClamps(t *testing.T) {
	t.Parallel()
	var raw rawStats
	input := `{
		"read":"2026-09-09T12:00:00Z",
		"cpu_stats":{"cpu_usage":{"total_usage":1100,"percpu_usage":[1,1]},"system_cpu_usage":1100,"online_cpus":2},
		"precpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":1000},
		"memory_stats":{"usage":1200,"limit":1000,"stats":{"inactive_file":200}}
	}`
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatal(err)
	}
	stats := sanitizeStats(raw)
	if stats.CPUPercent != 200 {
		t.Fatalf("CPU percent = %f, want clamped 200", stats.CPUPercent)
	}
	if stats.MemoryUsage != 1000 || stats.MemoryPercent != 100 {
		t.Fatalf("cache-adjusted memory stats = %+v, want 1000 bytes and 100%%", stats)
	}
	wantReadAt := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC).Unix()
	if stats.ReadAt != wantReadAt {
		t.Fatalf("ReadAt = %d, want %d", stats.ReadAt, wantReadAt)
	}
}

func TestSanitizeStatsPreventsUnsignedUnderflow(t *testing.T) {
	t.Parallel()
	var raw rawStats
	input := `{
		"cpu_stats":{"cpu_usage":{"total_usage":10,"percpu_usage":[1,1]},"system_cpu_usage":20,"online_cpus":2},
		"precpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":200},
		"memory_stats":{"usage":100,"limit":1000,"stats":{"inactive_file":200}}
	}`
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatal(err)
	}
	stats := sanitizeStats(raw)
	if stats.CPUPercent != 0 || stats.MemoryUsage != 0 || stats.MemoryPercent != 0 {
		t.Fatalf("counter reset/cache underflow was not handled: %+v", stats)
	}
}

func TestSanitizeStatsUsesCgroupV1TotalInactiveFile(t *testing.T) {
	t.Parallel()
	var raw rawStats
	input := `{
		"cpu_stats":{},
		"precpu_stats":{},
		"memory_stats":{"usage":1000,"limit":2000,"stats":{"inactive_file":100,"total_inactive_file":300}}
	}`
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatal(err)
	}
	stats := sanitizeStats(raw)
	if stats.MemoryUsage != 700 || stats.MemoryPercent != 35 {
		t.Fatalf("cgroup v1 working set = %d bytes (%.1f%%), want 700 bytes (35%%)", stats.MemoryUsage, stats.MemoryPercent)
	}
}

func dockerLogFrame(stream byte, payload []byte) []byte {
	frame := make([]byte, 8+len(payload))
	frame[0] = stream
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[8:], payload)
	return frame
}

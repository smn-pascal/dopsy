package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/smn-pascal/dopsy/internal/domain"
)

type overviewGateway struct {
	list    func(context.Context) ([]domain.Container, error)
	inspect func(context.Context, string) (domain.Inspection, error)
	stats   func(context.Context, string) (domain.Stats, error)
	logs    atomic.Int32
}

func (gateway *overviewGateway) Mode() string                { return "demo" }
func (gateway *overviewGateway) Check(context.Context) error { return nil }
func (gateway *overviewGateway) ListContainers(ctx context.Context) ([]domain.Container, error) {
	return gateway.list(ctx)
}
func (gateway *overviewGateway) InspectContainer(ctx context.Context, id string) (domain.Inspection, error) {
	return gateway.inspect(ctx, id)
}
func (gateway *overviewGateway) ContainerStats(ctx context.Context, id string) (domain.Stats, error) {
	return gateway.stats(ctx, id)
}
func (gateway *overviewGateway) ContainerLogs(context.Context, string, domain.LogOptions) (domain.Logs, error) {
	gateway.logs.Add(1)
	return domain.Logs{}, errors.New("overview must not read logs")
}

func (gateway *overviewGateway) ContainerEvents(context.Context, string, domain.EventOptions) (domain.Events, error) {
	return domain.Events{}, errors.New("overview must not read events")
}

func readOverview(t *testing.T, handler http.Handler) (int, domain.Overview) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var body domain.Overview
	if response.Code == http.StatusOK {
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
	}
	return response.Code, body
}

func TestOverviewDemoContract(t *testing.T) {
	t.Parallel()
	code, body := readOverview(t, testHandler(t))
	if code != http.StatusOK || body.GeneratedAt <= 0 {
		t.Fatalf("status=%d generatedAt=%d", code, body.GeneratedAt)
	}
	if body.Summary != (domain.OverviewSummary{Total: 2, Running: 1, Healthy: 1, NeedsReview: 1}) {
		t.Fatalf("summary=%+v", body.Summary)
	}
	if body.Collection != (domain.OverviewCollection{Observed: 2, Total: 2}) {
		t.Fatalf("collection=%+v", body.Collection)
	}
	if len(body.Containers) != 2 {
		t.Fatalf("containers=%d", len(body.Containers))
	}
	for _, item := range body.Containers {
		switch item.ID {
		case "demo-api":
			if item.Details == nil || !item.Details.OOMKilled || item.Metrics != nil || item.Assessment.Level != "critical" {
				t.Fatalf("stopped container=%+v", item)
			}
		case "demo-database":
			if item.Details == nil || !item.Details.Running || item.Metrics == nil || item.Assessment.Level != "ok" {
				t.Fatalf("running container=%+v", item)
			}
		default:
			t.Fatalf("unexpected container=%+v", item)
		}
	}
}

func TestOverviewIsBoundedAndPrioritisesCurrentSignals(t *testing.T) {
	t.Parallel()
	var inspected atomic.Int32
	var statsRead atomic.Int32
	listed := make([]domain.Container, 55)
	for index := range listed {
		listed[index] = domain.Container{
			ID: fmt.Sprintf("container-%02d", index), Name: fmt.Sprintf("service-%02d", index),
			State: "running", Status: "Up", Created: 1,
		}
	}
	listed[54].State = "exited"
	listed[54].Status = "Exited (0)"
	gateway := &overviewGateway{
		list: func(context.Context) ([]domain.Container, error) { return listed, nil },
		inspect: func(_ context.Context, id string) (domain.Inspection, error) {
			inspected.Add(1)
			return domain.Inspection{ID: id, Running: id != "container-54", Status: listedState(id)}, nil
		},
		stats: func(_ context.Context, _ string) (domain.Stats, error) {
			statsRead.Add(1)
			return domain.Stats{CPUPercent: 2, MemoryUsage: 1, MemoryLimit: 10, MemoryPercent: 10, ReadAt: 1}, nil
		},
	}
	code, body := readOverview(t, New(gateway, nil, Options{}))
	if code != http.StatusOK || !body.Collection.Truncated || body.Collection.Partial || body.Collection.Observed != 50 || body.Collection.Total != 55 {
		t.Fatalf("status=%d collection=%+v", code, body.Collection)
	}
	if body.Summary.Total != 55 || body.Summary.Running != 54 || body.Summary.NeedsReview != 1 {
		t.Fatalf("summary=%+v", body.Summary)
	}
	if inspected.Load() != 50 || statsRead.Load() != 49 || gateway.logs.Load() != 0 {
		t.Fatalf("inspect=%d stats=%d logs=%d", inspected.Load(), statsRead.Load(), gateway.logs.Load())
	}
	if body.Containers[0].ID != "container-54" || body.Containers[0].Metrics != nil {
		t.Fatalf("exited container was not sampled first: %+v", body.Containers[0])
	}
}

func listedState(id string) string {
	if id == "container-54" {
		return "exited"
	}
	return "running"
}

func TestOverviewReturnsPartialFactsWithoutInventedMetrics(t *testing.T) {
	t.Parallel()
	gateway := &overviewGateway{
		list: func(context.Context) ([]domain.Container, error) {
			return []domain.Container{
				{ID: "one", Name: "one", State: "running"},
				{ID: "two", Name: "two", State: "running"},
			}, nil
		},
		inspect: func(_ context.Context, id string) (domain.Inspection, error) {
			if id == "one" {
				return domain.Inspection{}, errors.New("inspection failed")
			}
			return domain.Inspection{ID: id, Running: true, Status: "running"}, nil
		},
		stats: func(context.Context, string) (domain.Stats, error) {
			return domain.Stats{}, errors.New("stats failed")
		},
	}
	code, body := readOverview(t, New(gateway, nil, Options{}))
	if code != http.StatusOK || !body.Collection.Partial || body.Collection.Truncated || body.Collection.Observed != 2 {
		t.Fatalf("status=%d collection=%+v", code, body.Collection)
	}
	for _, item := range body.Containers {
		if item.Assessment.Level != "unknown" || item.Metrics != nil {
			t.Fatalf("missing values were invented: %+v", item)
		}
		if item.ID == "one" && item.Details != nil {
			t.Fatalf("failed inspection returned details: %+v", item)
		}
		if item.ID == "two" && item.Details == nil {
			t.Fatalf("successful inspection omitted details: %+v", item)
		}
	}
	if gateway.logs.Load() != 0 {
		t.Fatalf("overview read %d logs", gateway.logs.Load())
	}
}

func TestOverviewRejectsListFailureAndUnsupportedMethod(t *testing.T) {
	t.Parallel()
	gateway := &overviewGateway{
		list: func(context.Context) ([]domain.Container, error) { return nil, errors.New("Docker unavailable") },
	}
	handler := New(gateway, nil, Options{})
	code, _ := readOverview(t, handler)
	if code != http.StatusBadGateway {
		t.Fatalf("list failure status=%d", code)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/overview", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET" {
		t.Fatalf("method status=%d allow=%q", response.Code, response.Header().Get("Allow"))
	}
}

func TestOverviewReadsAtMostFourContainersConcurrently(t *testing.T) {
	t.Parallel()
	var active atomic.Int32
	var maximum atomic.Int32
	listed := make([]domain.Container, 20)
	for index := range listed {
		listed[index] = domain.Container{ID: fmt.Sprintf("id-%02d", index), Name: fmt.Sprintf("name-%02d", index), State: "exited"}
	}
	gateway := &overviewGateway{
		list: func(context.Context) ([]domain.Container, error) { return listed, nil },
		inspect: func(ctx context.Context, id string) (domain.Inspection, error) {
			current := active.Add(1)
			for {
				old := maximum.Load()
				if current <= old || maximum.CompareAndSwap(old, current) {
					break
				}
			}
			defer active.Add(-1)
			select {
			case <-time.After(10 * time.Millisecond):
			case <-ctx.Done():
				return domain.Inspection{}, ctx.Err()
			}
			return domain.Inspection{ID: id, Status: "exited"}, nil
		},
	}
	code, body := readOverview(t, New(gateway, nil, Options{}))
	if code != http.StatusOK || body.Collection.Partial || maximum.Load() > 4 || maximum.Load() < 2 {
		t.Fatalf("status=%d partial=%v max-concurrent=%d", code, body.Collection.Partial, maximum.Load())
	}
}

func TestOverviewRespectsRequestDeadlineAndReturnsPartialListFacts(t *testing.T) {
	t.Parallel()
	gateway := &overviewGateway{
		list: func(context.Context) ([]domain.Container, error) {
			return []domain.Container{{ID: "slow", Name: "slow", State: "running"}}, nil
		},
		inspect: func(ctx context.Context, _ string) (domain.Inspection, error) {
			<-ctx.Done()
			return domain.Inspection{}, ctx.Err()
		},
	}
	start := time.Now()
	code, body := readOverview(t, New(gateway, nil, Options{RequestTimeout: 30 * time.Millisecond}))
	if time.Since(start) > time.Second || code != http.StatusOK || !body.Collection.Partial || len(body.Containers) != 1 || body.Containers[0].Details != nil {
		t.Fatalf("elapsed=%s status=%d body=%+v", time.Since(start), code, body)
	}
}

func TestOverviewDoesNotExposeInspectionErrorOrUnboundedDockerText(t *testing.T) {
	t.Parallel()
	gateway := &overviewGateway{
		list: func(context.Context) ([]domain.Container, error) {
			return []domain.Container{{
				ID: "safe-id", Name: "safe\n" + strings.Repeat("x", 10000),
				Image: strings.Repeat("i", 10000), State: "running", Status: strings.Repeat("s", 10000),
			}}, nil
		},
		inspect: func(context.Context, string) (domain.Inspection, error) {
			return domain.Inspection{Running: true, Status: "running", Error: "DO_NOT_EXPOSE_INSPECTION_ERROR"}, nil
		},
		stats: func(context.Context, string) (domain.Stats, error) {
			return domain.Stats{CPUPercent: 1, MemoryPercent: 1}, nil
		},
	}
	request := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	response := httptest.NewRecorder()
	New(gateway, nil, Options{}).ServeHTTP(response, request)
	if response.Code != http.StatusOK || len(response.Body.Bytes()) > maxOverviewResponseBytes || strings.Contains(response.Body.String(), "DO_NOT_EXPOSE") {
		t.Fatalf("status=%d bytes=%d body=%s", response.Code, len(response.Body.Bytes()), response.Body.String())
	}
	var body domain.Overview
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	item := body.Containers[0]
	if len(item.Name) > 128 || len(item.Image) > 256 || len(item.Status) > 128 || strings.Contains(item.Name, "\n") {
		t.Fatalf("Docker text was not bounded: %+v", item)
	}
}

func TestOverviewSkipsMalformedContainerIDs(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	gateway := &overviewGateway{
		list: func(context.Context) ([]domain.Container, error) {
			return []domain.Container{{ID: "../../bad", Name: "bad", State: "running"}}, nil
		},
		inspect: func(context.Context, string) (domain.Inspection, error) {
			calls.Add(1)
			return domain.Inspection{}, nil
		},
	}
	code, body := readOverview(t, New(gateway, nil, Options{}))
	if code != http.StatusOK || !body.Collection.Partial || body.Collection.Total != 1 || body.Collection.Observed != 0 || len(body.Containers) != 0 || calls.Load() != 0 {
		t.Fatalf("status=%d body=%+v inspect-calls=%d", code, body, calls.Load())
	}
}

func TestOverviewAssessmentUsesOnlyDeterministicCurrentFacts(t *testing.T) {
	t.Parallel()
	unhealthy := "unhealthy"
	starting := "starting"
	tests := []struct {
		name    string
		view    domain.ContainerOverview
		details *domain.ContainerDetails
		missing bool
		want    string
	}{
		{name: "OOM", view: domain.ContainerOverview{State: "running"}, details: &domain.ContainerDetails{Running: true, OOMKilled: true}, want: "critical"},
		{name: "unhealthy", view: domain.ContainerOverview{State: "running", Health: &unhealthy}, details: &domain.ContainerDetails{Running: true}, want: "critical"},
		{name: "non-zero exit", view: domain.ContainerOverview{State: "exited"}, details: &domain.ContainerDetails{ExitCode: 2}, want: "warning"},
		{name: "restart count", view: domain.ContainerOverview{State: "running"}, details: &domain.ContainerDetails{Running: true, RestartCount: 1}, want: "warning"},
		{name: "memory threshold", view: domain.ContainerOverview{State: "running", Metrics: &domain.Stats{MemoryLimit: 100, MemoryPercent: 90}}, details: &domain.ContainerDetails{Running: true}, want: "warning"},
		{name: "missing stats", view: domain.ContainerOverview{State: "running"}, details: &domain.ContainerDetails{Running: true}, missing: true, want: "unknown"},
		{name: "starting health", view: domain.ContainerOverview{State: "running", Health: &starting}, details: &domain.ContainerDetails{Running: true}, want: "unknown"},
		{name: "no health check is not unhealthy", view: domain.ContainerOverview{State: "running"}, details: &domain.ContainerDetails{Running: true}, want: "ok"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := assessOverview(test.view, test.details, test.missing)
			if got.Level != test.want || got.Summary == "" {
				t.Fatalf("assessment=%+v, want level %q", got, test.want)
			}
		})
	}
}

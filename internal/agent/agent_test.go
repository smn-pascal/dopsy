package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/smn-pascal/dopsy/internal/domain"
	"github.com/smn-pascal/dopsy/internal/session"
)

type fakeProvider struct {
	responses []CompletionResponse
	requests  []CompletionRequest
	calls     int
}

func (f *fakeProvider) Complete(_ context.Context, request CompletionRequest) (CompletionResponse, error) {
	f.requests = append(f.requests, request)
	if f.calls >= len(f.responses) {
		return CompletionResponse{}, errors.New("unexpected provider call")
	}
	response := f.responses[f.calls]
	f.calls++
	return response, nil
}

type repeatingProvider struct {
	calls int
}

func (f *repeatingProvider) Complete(context.Context, CompletionRequest) (CompletionResponse, error) {
	f.calls++
	return CompletionResponse{ToolCalls: []ToolCall{{
		ID: "stats", Name: "get_container_stats", Arguments: `{"containerId":"demo-api"}`,
	}}}, nil
}

type fakeGateway struct {
	logsOptions domain.LogOptions
	logsText    string
	statsCalls  int
}

func (f *fakeGateway) Mode() string                { return "demo" }
func (f *fakeGateway) Check(context.Context) error { return nil }
func (f *fakeGateway) ListContainers(context.Context) ([]domain.Container, error) {
	return []domain.Container{{ID: "demo-api", Name: "demo-api", State: "exited", Status: "Exited (137)"}}, nil
}
func (f *fakeGateway) InspectContainer(context.Context, string) (domain.Inspection, error) {
	return domain.Inspection{
		ID: "demo-api", Name: "demo-api", Status: "exited", OOMKilled: true,
		ExitCode: 137, RestartCount: 3, MemoryLimit: 256 * 1024 * 1024,
	}, nil
}
func (f *fakeGateway) ContainerLogs(_ context.Context, _ string, options domain.LogOptions) (domain.Logs, error) {
	f.logsOptions = options
	text := f.logsText
	if text == "" {
		text = strings.Repeat("x", 4096) + " out of memory"
	}
	return domain.Logs{Text: text, Tail: options.Tail}, nil
}
func (f *fakeGateway) ContainerStats(context.Context, string) (domain.Stats, error) {
	f.statsCalls++
	return domain.Stats{CPUPercent: 4.2, MemoryUsage: 250, MemoryLimit: 256, MemoryPercent: 97.6}, nil
}

func TestAgentRunsMultiStepToolLoop(t *testing.T) {
	t.Parallel()
	gateway := &fakeGateway{}
	provider := &fakeProvider{responses: []CompletionResponse{
		{ToolCalls: []ToolCall{{ID: "inspect-1", Name: "inspect_container", Arguments: `{"containerId":"demo-api"}`}}},
		{ToolCalls: []ToolCall{{ID: "logs-1", Name: "get_container_logs", Arguments: `{"containerId":"demo-api","tail":100}`}}},
		{Content: "The container exceeded its memory limit and was killed."},
	}}
	agent := New(gateway, provider, Options{MaxRounds: 6, MaxLogLines: 500, MaxLogBytes: 2048, Now: func() time.Time {
		return time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	}})
	diagnosis, err := agent.Diagnose(context.Background(), "Why did it restart?", "demo-api")
	if err != nil {
		t.Fatal(err)
	}
	if diagnosis.Answer == "" || len(diagnosis.Steps) != 2 {
		t.Fatalf("unexpected diagnosis: %+v", diagnosis)
	}
	if len(diagnosis.Evidence) < 3 {
		t.Fatalf("expected inspection evidence, got %+v", diagnosis.Evidence)
	}
	if provider.calls != 3 {
		t.Fatalf("provider calls = %d, want 3", provider.calls)
	}
	if got := provider.requests[1].Messages[len(provider.requests[1].Messages)-1]; got.Role != "tool" || got.ToolCallID != "inspect-1" || !strings.Contains(got.Content, `"oomKilled":true`) {
		t.Fatalf("inspection result was not returned to provider safely: %+v", got)
	}
}

func TestAgentFallbackDiagnosesDemoOOM(t *testing.T) {
	t.Parallel()
	agent := New(&fakeGateway{}, nil, Options{})
	diagnosis, err := agent.Diagnose(context.Background(), "Why did it stop?", "demo-api")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(diagnosis.Answer), "out of memory") {
		t.Fatalf("answer = %q, want deterministic OOM diagnosis", diagnosis.Answer)
	}
	if len(diagnosis.Steps) != 2 {
		t.Fatalf("steps = %+v, want inspect and logs", diagnosis.Steps)
	}
}

func TestAgentHardCapsRounds(t *testing.T) {
	t.Parallel()
	provider := &repeatingProvider{}
	agent := New(&fakeGateway{}, provider, Options{MaxRounds: 100})
	_, err := agent.Diagnose(context.Background(), "Keep looking", "demo-api")
	if !errors.Is(err, ErrRoundLimit) {
		t.Fatalf("error = %v, want %v", err, ErrRoundLimit)
	}
	if provider.calls != hardMaxRounds {
		t.Fatalf("provider calls = %d, want hard cap %d", provider.calls, hardMaxRounds)
	}
}

func TestAgentClampsAndTruncatesLogs(t *testing.T) {
	t.Parallel()
	gateway := &fakeGateway{}
	provider := &fakeProvider{responses: []CompletionResponse{
		{ToolCalls: []ToolCall{{ID: "logs", Name: "get_container_logs", Arguments: `{"containerId":"demo-api","tail":999999}`}}},
		{Content: "Done."},
	}}
	agent := New(gateway, provider, Options{MaxLogLines: 50, MaxLogBytes: 1024})
	if _, err := agent.Diagnose(context.Background(), "Check logs", "demo-api"); err != nil {
		t.Fatal(err)
	}
	if gateway.logsOptions.Tail != 50 {
		t.Fatalf("tail = %d, want configured cap 50", gateway.logsOptions.Tail)
	}
	toolResult := provider.requests[1].Messages[len(provider.requests[1].Messages)-1].Content
	if len(toolResult) > 1200 || !strings.Contains(toolResult, `"truncated":true`) {
		t.Fatalf("tool result was not bounded: len=%d body=%s", len(toolResult), toolResult)
	}
}

func TestAgentRejectsContainerOutsideScope(t *testing.T) {
	t.Parallel()
	provider := &fakeProvider{responses: []CompletionResponse{
		{ToolCalls: []ToolCall{{ID: "inspect", Name: "inspect_container", Arguments: `{"containerId":"other"}`}}},
		{Content: "The selected container could not be inspected."},
		{Content: "No grounded diagnosis is possible."},
	}}
	agent := New(&fakeGateway{}, provider, Options{})
	diagnosis, err := agent.Diagnose(context.Background(), "Inspect it", "demo-api")
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnosis.Steps) != 1 || diagnosis.Steps[0].Summary != "Request rejected or unavailable" {
		t.Fatalf("out-of-scope call was not rejected: %+v", diagnosis.Steps)
	}
}

func TestAgentAddsBoundedConversationHistoryAndTimezone(t *testing.T) {
	t.Parallel()
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	provider := &fakeProvider{responses: []CompletionResponse{
		{ToolCalls: []ToolCall{{ID: "inspect", Name: "inspect_container", Arguments: `{"containerId":"demo-api"}`}}},
		{Content: "The current evidence confirms the earlier hypothesis."},
	}}
	agent := New(&fakeGateway{}, provider, Options{
		TimezoneName: "Europe/Berlin",
		Location:     location,
		Now: func() time.Time {
			return time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
		},
	})
	history := []session.Turn{{User: "Why did it restart?", Assistant: "Memory pressure was a possibility."}}
	if _, err := agent.DiagnoseWithHistory(context.Background(), "Can you verify that?", "demo-api", history); err != nil {
		t.Fatal(err)
	}
	first := provider.requests[0].Messages
	if len(first) != 4 || first[1].Content != history[0].User || first[2].Content != history[0].Assistant || first[3].Content != "Can you verify that?" {
		t.Fatalf("provider history = %+v", first)
	}
	if !strings.Contains(first[0].Content, `Europe/Berlin`) || !strings.Contains(first[0].Content, "2026-09-09T14:00:00+02:00") || !strings.Contains(first[0].Content, "2026-09-09T12:00:00Z") {
		t.Fatalf("timezone context missing from system prompt: %s", first[0].Content)
	}
}

func TestAgentDeduplicatesEquivalentToolCalls(t *testing.T) {
	t.Parallel()
	gateway := &fakeGateway{}
	provider := &fakeProvider{responses: []CompletionResponse{
		{ToolCalls: []ToolCall{
			{ID: "first", Name: "get_container_stats", Arguments: `{"containerId":"demo-api"}`},
			{ID: "second", Name: "get_container_stats", Arguments: `{ "containerId" : "demo-api" }`},
		}},
		{Content: "The current memory snapshot is elevated."},
	}}
	diagnosis, err := New(gateway, provider, Options{}).Diagnose(context.Background(), "Check it", "demo-api")
	if err != nil {
		t.Fatal(err)
	}
	if gateway.statsCalls != 1 {
		t.Fatalf("stats calls = %d, want 1", gateway.statsCalls)
	}
	if len(diagnosis.Steps) != 2 || diagnosis.Steps[1].Summary != "Duplicate request rejected" {
		t.Fatalf("duplicate step was not recorded safely: %+v", diagnosis.Steps)
	}
}

func TestAgentCapsTotalToolCalls(t *testing.T) {
	t.Parallel()
	provider := &fakeProvider{responses: []CompletionResponse{{ToolCalls: []ToolCall{
		{ID: "one", Name: "get_container_stats", Arguments: `{"containerId":"demo-api"}`},
		{ID: "two", Name: "inspect_container", Arguments: `{"containerId":"demo-api"}`},
	}}}}
	_, err := New(&fakeGateway{}, provider, Options{MaxToolCalls: 1}).Diagnose(context.Background(), "Inspect", "demo-api")
	if !errors.Is(err, ErrToolCallLimit) {
		t.Fatalf("error = %v, want %v", err, ErrToolCallLimit)
	}
}

func TestAgentCapsAggregateToolOutput(t *testing.T) {
	t.Parallel()
	gateway := &fakeGateway{logsText: strings.Repeat("large-log-line\n", 4000)}
	provider := &fakeProvider{responses: []CompletionResponse{
		{ToolCalls: []ToolCall{{ID: "logs", Name: "get_container_logs", Arguments: `{"containerId":"demo-api","tail":2000}`}}},
		{ToolCalls: []ToolCall{{ID: "inspect", Name: "inspect_container", Arguments: `{"containerId":"demo-api"}`}}},
		{Content: "The bounded evidence indicates an OOM termination."},
	}}
	diagnosis, err := New(gateway, provider, Options{MaxToolOutputBytes: 16 * 1024}).Diagnose(context.Background(), "Inspect", "demo-api")
	if err != nil {
		t.Fatal(err)
	}
	firstToolResult := provider.requests[1].Messages[len(provider.requests[1].Messages)-1].Content
	if !strings.Contains(firstToolResult, "remaining run budget") || len(firstToolResult) >= 16*1024 {
		t.Fatalf("oversized tool output was not replaced: len=%d body=%q", len(firstToolResult), firstToolResult)
	}
	if diagnosis.Answer == "" {
		t.Fatal("successful bounded follow-up did not produce a diagnosis")
	}
}

func TestAgentRejectsUngroundedProviderAnswer(t *testing.T) {
	t.Parallel()
	provider := &fakeProvider{responses: []CompletionResponse{
		{Content: "I know without looking."},
		{Content: "Still no need to inspect Docker."},
	}}
	diagnosis, err := New(&fakeGateway{}, provider, Options{}).Diagnose(context.Background(), "What happened?", "demo-api")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(diagnosis.Answer, "Still no need") || !strings.Contains(diagnosis.Answer, "no Docker evidence") {
		t.Fatalf("ungrounded model answer escaped: %q", diagnosis.Answer)
	}
}

func TestFallbackDoesNotInventLogEvidenceOrMatchRoomAsOOM(t *testing.T) {
	t.Parallel()
	gateway := &fakeGateway{logsText: "There is room to grow; zoom level is normal."}
	diagnosis, err := New(gateway, nil, Options{}).Diagnose(context.Background(), "Why did it stop?", "demo-api")
	if err != nil {
		t.Fatal(err)
	}
	for _, evidence := range diagnosis.Evidence {
		if evidence.Label == "Log signal" {
			t.Fatalf("ordinary word was misclassified as OOM evidence: %+v", diagnosis.Evidence)
		}
	}
	if strings.Contains(strings.ToLower(diagnosis.Answer), "logs independently") {
		t.Fatalf("fallback invented log support: %q", diagnosis.Answer)
	}
}

func TestAgentRejectsUnboundedHistoricalLogWindows(t *testing.T) {
	t.Parallel()
	diagnoser := New(&fakeGateway{}, nil, Options{})
	tests := []ToolCall{
		{Name: "get_container_logs", Arguments: `{"containerId":"demo-api","tail":100,"since":"2026-09-09T00:00:00Z"}`},
		{Name: "get_container_logs", Arguments: `{"containerId":"demo-api","tail":100,"since":"2026-09-07T00:00:00Z","until":"2026-09-09T00:00:01Z"}`},
	}
	for _, call := range tests {
		result, _, step, success := diagnoser.executeTool(context.Background(), "demo-api", call)
		if success || !strings.Contains(result, `"error"`) || step.Summary != "Request rejected or unavailable" {
			t.Fatalf("unbounded window was not rejected: success=%v result=%s step=%+v", success, result, step)
		}
	}
}

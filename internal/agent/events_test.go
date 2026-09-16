package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/smn-pascal/dopsy/internal/domain"
)

func TestAgentEventToolReturnsFactsAndHistoryWarning(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	gateway := &fakeGateway{}
	provider := &fakeProvider{responses: []CompletionResponse{
		{ToolCalls: []ToolCall{{ID: "events", Name: "get_container_events", Arguments: `{"containerId":"demo-api"}`}}},
		{Content: "Docker retained an OOM event; the event history is incomplete."},
	}}
	agent := New(gateway, provider, Options{Now: func() time.Time { return now }})
	diagnosis, err := agent.Diagnose(context.Background(), "What happened?", "demo-api")
	if err != nil {
		t.Fatal(err)
	}
	if gateway.eventCalls != 1 || gateway.eventOptions.Until != now.Unix()-1 || gateway.eventOptions.Since != now.Unix()-3601 {
		t.Fatalf("calls=%d options=%+v", gateway.eventCalls, gateway.eventOptions)
	}
	if len(diagnosis.Evidence) != 2 || diagnosis.Evidence[0].Label != "Event history" || diagnosis.Evidence[1].Severity != "critical" {
		t.Fatalf("evidence=%+v", diagnosis.Evidence)
	}
	result := provider.requests[1].Messages[len(provider.requests[1].Messages)-1].Content
	if !strings.Contains(result, `"historyLimited":true`) || !strings.Contains(result, `"action":"oom"`) {
		t.Fatalf("tool result=%s", result)
	}
	found := false
	for _, tool := range provider.requests[0].Tools {
		if tool.Name == "get_container_events" {
			found = true
		}
	}
	if !found {
		t.Fatal("provider did not receive event tool")
	}
}

func TestDemoEventReadFitsCallBudget(t *testing.T) {
	t.Parallel()
	gateway := &fakeGateway{}
	agent := New(gateway, nil, Options{MaxToolCalls: 2})
	if _, err := agent.Diagnose(context.Background(), "Why did it stop?", "demo-api"); err != nil {
		t.Fatal(err)
	}
	if gateway.eventCalls != 0 {
		t.Fatal("optional event read exceeded call budget")
	}
}

func TestEmptyAndTruncatedEventEvidenceWarnsAboutMissingHistory(t *testing.T) {
	t.Parallel()
	for _, truncated := range []bool{false, true} {
		evidence := eventEvidence(domain.Events{Items: []domain.ContainerEvent{}, HistoryLimited: true, Truncated: truncated})
		if len(evidence) == 0 || !strings.Contains(evidence[0].Value, "not a complete archive") {
			t.Fatalf("evidence=%+v", evidence)
		}
		if truncated && len(evidence) != 2 {
			t.Fatalf("truncation warning missing: %+v", evidence)
		}
	}
}

func TestAgentEventToolRejectsScopeAndUnsafeWindows(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	tests := []string{
		`{"containerId":"other"}`,
		`{"containerId":"demo-api","since":"2026-09-16T10:00:00Z"}`,
		`{"containerId":"demo-api","until":"2026-09-16T11:00:00Z"}`,
		`{"containerId":"demo-api","since":"bad","until":"2026-09-16T11:00:00Z"}`,
		`{"containerId":"demo-api","since":"2026-09-16T11:00:00Z","until":"2026-09-16T12:00:00Z"}`,
		`{"containerId":"demo-api","since":"2026-09-14T11:00:00Z","until":"2026-09-16T11:00:00Z"}`,
		`{"containerId":"demo-api","since":"2026-09-16T11:00:00Z","until":"2026-09-16T10:00:00Z"}`,
		`{"containerId":"demo-api","labels":true}`,
	}
	for _, arguments := range tests {
		t.Run(arguments, func(t *testing.T) {
			gateway := &fakeGateway{}
			agent := New(gateway, nil, Options{Now: func() time.Time { return now }})
			content, evidence, _, success := agent.executeTool(context.Background(), "demo-api", ToolCall{Name: "get_container_events", Arguments: arguments})
			if success || gateway.eventCalls != 0 || len(evidence) != 0 || !strings.Contains(content, `"error"`) {
				t.Fatalf("success=%v calls=%d content=%s", success, gateway.eventCalls, content)
			}
		})
	}
}

func TestAgentEventToolAcceptsExplicitTimezoneWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	gateway := &fakeGateway{}
	agent := New(gateway, nil, Options{Now: func() time.Time { return now }})
	_, _, _, success := agent.executeTool(context.Background(), "demo-api", ToolCall{Name: "get_container_events", Arguments: `{"containerId":"demo-api","since":"2026-09-16T11:00:00+02:00","until":"2026-09-16T12:00:00+02:00"}`})
	if !success || gateway.eventOptions.Since != now.Add(-3*time.Hour).Unix() || gateway.eventOptions.Until != now.Add(-2*time.Hour).Unix() {
		t.Fatalf("success=%v window=%+v", success, gateway.eventOptions)
	}
}

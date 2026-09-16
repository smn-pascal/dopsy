package dockergateway

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/smn-pascal/dopsy/internal/domain"
)

func TestDemoLogsMatchInspectionAndEventTimes(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	demo := NewDemo()
	demo.now = func() time.Time { return now }
	inspection, err := demo.InspectContainer(context.Background(), "demo-api")
	if err != nil {
		t.Fatal(err)
	}
	logs, err := demo.ContainerLogs(context.Background(), "demo-api", domain.LogOptions{Tail: 200})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.Text, inspection.FinishedAt+" INFO process terminated") {
		t.Fatalf("finished=%s logs=%s", inspection.FinishedAt, logs.Text)
	}
	events, err := demo.ContainerEvents(context.Background(), "demo-api", domain.EventOptions{Since: now.Add(-time.Hour).Unix(), Until: now.Add(-time.Second).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events.Items {
		if event.Action == "oom" && !strings.Contains(logs.Text, time.Unix(event.Time, 0).UTC().Format(time.RFC3339)+" ERROR allocation failed") {
			t.Fatalf("OOM event and log do not align: %+v", event)
		}
	}
}

func TestDemoLogsApplyWindowAndTail(t *testing.T) {
	t.Parallel()
	now := time.Unix(200000, 0)
	demo := NewDemo()
	demo.now = func() time.Time { return now }
	empty, err := demo.ContainerLogs(context.Background(), "demo-api", domain.LogOptions{Tail: 200, Since: now.Unix() - 100, Until: now.Unix() - 1})
	if err != nil || empty.Text != "" {
		t.Fatalf("unrelated time window returned logs: %+v err=%v", empty, err)
	}
	last, err := demo.ContainerLogs(context.Background(), "demo-api", domain.LogOptions{Tail: 1})
	if err != nil || strings.Count(last.Text, "\n") != 1 || !strings.Contains(last.Text, "process terminated") {
		t.Fatalf("tail=%+v err=%v", last, err)
	}
	before, err := demo.ContainerLogs(context.Background(), "demo-api", domain.LogOptions{Tail: 200, Until: now.Unix() - 242})
	if err != nil || strings.Contains(before.Text, "out of memory") || !strings.Contains(before.Text, "WARN") {
		t.Fatalf("until=%+v err=%v", before, err)
	}
}

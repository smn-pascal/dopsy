package dockergateway

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/smn-pascal/dopsy/internal/domain"
)

type DemoGateway struct {
	now func() time.Time
}

func NewDemo() *DemoGateway {
	return &DemoGateway{now: time.Now}
}

func (d *DemoGateway) Mode() string { return "demo" }

func (d *DemoGateway) Check(context.Context) error { return nil }

func (d *DemoGateway) ListContainers(context.Context) ([]domain.Container, error) {
	healthy := "healthy"
	return []domain.Container{
		{
			ID:      "demo-api",
			Name:    "demo-api",
			Image:   "ghcr.io/smn-pascal/dopsy-demo-api:latest",
			State:   "exited",
			Status:  "Exited (137) 4 minutes ago",
			Created: d.now().Add(-48 * time.Hour).Unix(),
		},
		{
			ID:      "demo-database",
			Name:    "demo-database",
			Image:   "postgres:17-alpine",
			State:   "running",
			Status:  "Up 2 days (healthy)",
			Health:  &healthy,
			Created: d.now().Add(-72 * time.Hour).Unix(),
		},
	}, nil
}

func (d *DemoGateway) InspectContainer(_ context.Context, id string) (domain.Inspection, error) {
	if !validContainerID(id) {
		return domain.Inspection{}, ErrInvalidContainer
	}
	now := d.now()
	switch strings.TrimPrefix(id, "/") {
	case "demo-api":
		return domain.Inspection{
			ID:           "demo-api",
			Name:         "demo-api",
			Image:        "ghcr.io/smn-pascal/dopsy-demo-api:latest",
			Status:       "exited",
			Running:      false,
			OOMKilled:    true,
			ExitCode:     137,
			StartedAt:    now.Add(-37 * time.Minute).UTC().Format(time.RFC3339),
			FinishedAt:   now.Add(-4 * time.Minute).UTC().Format(time.RFC3339),
			RestartCount: 3,
			MemoryLimit:  256 * 1024 * 1024,
		}, nil
	case "demo-database":
		health := "healthy"
		return domain.Inspection{
			ID:          "demo-database",
			Name:        "demo-database",
			Image:       "postgres:17-alpine",
			Status:      "running",
			Running:     true,
			StartedAt:   now.Add(-48 * time.Hour).UTC().Format(time.RFC3339),
			Health:      &health,
			MemoryLimit: 512 * 1024 * 1024,
		}, nil
	default:
		return domain.Inspection{}, fmt.Errorf("container %q not found", id)
	}
}

func (d *DemoGateway) ContainerLogs(_ context.Context, id string, options domain.LogOptions) (domain.Logs, error) {
	if !validContainerID(id) {
		return domain.Logs{}, ErrInvalidContainer
	}
	tail := clampTail(options.Tail)
	now := d.now()
	type entry struct {
		at   time.Time
		text string
	}
	var entries []entry
	switch id {
	case "demo-api":
		stopped := now.Add(-4 * time.Minute)
		entries = []entry{
			{stopped.Add(-3 * time.Second), "WARN memory usage reached 248 MiB of 256 MiB"},
			{stopped.Add(-time.Second), "ERROR allocation failed: JavaScript heap out of memory"},
			{stopped, "INFO process terminated with signal SIGKILL"},
		}
	case "demo-database":
		entries = []entry{{now.Add(-48 * time.Hour), "LOG database system is ready to accept connections"}}
	default:
		return domain.Logs{}, fmt.Errorf("container %q not found", id)
	}
	lines := make([]string, 0, len(entries))
	for _, item := range entries {
		if options.Since > 0 && item.at.Unix() < options.Since {
			continue
		}
		if options.Until > 0 && item.at.Unix() > options.Until {
			continue
		}
		lines = append(lines, item.at.UTC().Format(time.RFC3339)+" "+item.text)
	}
	if len(lines) > tail {
		lines = lines[len(lines)-tail:]
	}
	text := strings.Join(lines, "\n")
	if text != "" {
		text += "\n"
	}
	return domain.Logs{Text: text, Tail: tail, Truncated: false}, nil
}

func (d *DemoGateway) ContainerStats(_ context.Context, id string) (domain.Stats, error) {
	if !validContainerID(id) {
		return domain.Stats{}, ErrInvalidContainer
	}
	switch id {
	case "demo-api":
		return domain.Stats{
			CPUPercent:    0,
			MemoryUsage:   248 * 1024 * 1024,
			MemoryLimit:   256 * 1024 * 1024,
			MemoryPercent: 96.875,
			ReadAt:        d.now().Unix(),
		}, nil
	case "demo-database":
		return domain.Stats{
			CPUPercent:    0.42,
			MemoryUsage:   86 * 1024 * 1024,
			MemoryLimit:   512 * 1024 * 1024,
			MemoryPercent: 16.796875,
			ReadAt:        d.now().Unix(),
		}, nil
	default:
		return domain.Stats{}, fmt.Errorf("container %q not found", id)
	}
}

func clampTail(tail int) int {
	if tail <= 0 {
		return 200
	}
	if tail > MaxLogTail {
		return MaxLogTail
	}
	return tail
}

func (d *DemoGateway) ContainerEvents(_ context.Context, id string, options domain.EventOptions) (domain.Events, error) {
	if !validContainerID(id) {
		return domain.Events{}, ErrInvalidContainer
	}
	if !validEventWindow(options, d.now().Unix()) {
		return domain.Events{}, fmt.Errorf("invalid event window")
	}
	if id != "demo-api" && id != "demo-database" {
		return domain.Events{}, fmt.Errorf("container not found")
	}
	result := domain.Events{Items: make([]domain.ContainerEvent, 0), Since: options.Since, Until: options.Until, HistoryLimited: true}
	if id == "demo-api" {
		at := d.now().Add(-4 * time.Minute).Unix()
		for _, event := range []domain.ContainerEvent{{Action: "start", Time: d.now().Add(-37 * time.Minute).Unix()}, {Action: "oom", Time: at - 1}, {Action: "die", Time: at}, {Action: "stop", Time: at + 1}} {
			if event.Time >= options.Since && event.Time <= options.Until {
				result.Items = append(result.Items, event)
			}
		}
	}
	return result, nil
}

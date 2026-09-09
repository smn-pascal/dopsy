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
	var text string
	switch id {
	case "demo-api":
		text = "2026-09-09T00:01:39Z WARN memory usage reached 248 MiB of 256 MiB\n" +
			"2026-09-09T00:01:41Z ERROR allocation failed: JavaScript heap out of memory\n" +
			"2026-09-09T00:01:42Z INFO process terminated with signal SIGKILL\n"
	case "demo-database":
		text = "2026-09-09T00:01:42Z LOG database system is ready to accept connections\n"
	default:
		return domain.Logs{}, fmt.Errorf("container %q not found", id)
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

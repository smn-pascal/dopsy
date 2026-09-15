package httpapi

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/smn-pascal/dopsy/internal/domain"
)

const (
	maxOverviewContainers     = 50
	maxOverviewConcurrent     = 4
	maxOverviewResponseBytes  = 128 * 1024
	overviewCollectionTimeout = 12 * time.Second
	overviewContainerTimeout  = 2 * time.Second
)

type overviewResult struct {
	index     int
	container domain.ContainerOverview
	partial   bool
}

func (a *API) overview(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), overviewCollectionTimeout)
	defer cancel()
	listed, err := a.docker.ListContainers(ctx)
	if err != nil {
		writeError(writer, http.StatusBadGateway, "Docker is not reachable")
		return
	}

	snapshot := domain.Overview{
		GeneratedAt: time.Now().Unix(),
		Summary:     domain.OverviewSummary{Total: len(listed)},
		Collection:  domain.OverviewCollection{Total: len(listed)},
		Containers:  make([]domain.ContainerOverview, 0),
	}
	valid := make([]domain.Container, 0, len(listed))
	for _, item := range listed {
		if strings.EqualFold(item.State, "running") {
			snapshot.Summary.Running++
		}
		if item.Health != nil && strings.EqualFold(*item.Health, "healthy") {
			snapshot.Summary.Healthy++
		}
		if !containerIDPattern.MatchString(item.ID) {
			// A malformed Docker ID is never passed to the gateway or returned
			// as a container-scoped investigation target.
			snapshot.Collection.Partial = true
			continue
		}
		valid = append(valid, item)
	}
	// When the list exceeds the observation cap, sample conspicuous current
	// signals first rather than depending on Docker's list order.
	sort.Slice(valid, func(i, j int) bool {
		left, right := overviewPriority(valid[i]), overviewPriority(valid[j])
		if left != right {
			return left < right
		}
		if valid[i].Name != valid[j].Name {
			return valid[i].Name < valid[j].Name
		}
		return valid[i].ID < valid[j].ID
	})
	if len(valid) > maxOverviewContainers {
		valid = valid[:maxOverviewContainers]
		snapshot.Collection.Truncated = true
	}
	snapshot.Collection.Observed = len(valid)
	snapshot.Containers = make([]domain.ContainerOverview, len(valid))
	for index, item := range valid {
		snapshot.Containers[index] = listedOverview(item)
	}

	if len(valid) > 0 {
		jobs := make(chan int)
		results := make(chan overviewResult, len(valid))
		for range min(len(valid), maxOverviewConcurrent) {
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case index, ok := <-jobs:
						if !ok {
							return
						}
						results <- a.collectOverviewContainer(ctx, index, valid[index], snapshot.Containers[index])
					}
				}
			}()
		}
		go func() {
			defer close(jobs)
			for index := range valid {
				select {
				case <-ctx.Done():
					return
				case jobs <- index:
				}
			}
		}()
		collected := 0
	collect:
		for collected < len(valid) {
			select {
			case result := <-results:
				snapshot.Containers[result.index] = result.container
				snapshot.Collection.Partial = snapshot.Collection.Partial || result.partial
				collected++
			case <-ctx.Done():
				snapshot.Collection.Partial = true
				break collect
			}
		}
	}
	for _, item := range snapshot.Containers {
		if item.Assessment.Level == "warning" || item.Assessment.Level == "critical" {
			snapshot.Summary.NeedsReview++
		}
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil || len(encoded) > maxOverviewResponseBytes {
		writeError(writer, http.StatusInternalServerError, "overview could not be encoded")
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(encoded)
}

func overviewPriority(item domain.Container) int {
	if item.Health != nil && strings.EqualFold(*item.Health, "unhealthy") {
		return 0
	}
	if !strings.EqualFold(item.State, "running") {
		return 1
	}
	if item.Health != nil && strings.EqualFold(*item.Health, "starting") {
		return 2
	}
	return 3
}

func listedOverview(item domain.Container) domain.ContainerOverview {
	name := overviewText(item.Name, 128)
	if name == "" {
		name = item.ID
	}
	view := domain.ContainerOverview{
		ID:      item.ID,
		Name:    name,
		Image:   overviewText(item.Image, 256),
		State:   overviewText(item.State, 32),
		Status:  overviewText(item.Status, 128),
		Created: item.Created,
	}
	view.Health = overviewHealth(item.Health)
	view.Assessment = assessOverview(view, nil, false)
	return view
}

func (a *API) collectOverviewContainer(parent context.Context, index int, item domain.Container, view domain.ContainerOverview) overviewResult {
	ctx, cancel := context.WithTimeout(parent, overviewContainerTimeout)
	defer cancel()
	result := overviewResult{index: index, container: view}
	inspection, err := a.docker.InspectContainer(ctx, item.ID)
	if err != nil {
		result.partial = true
		return result
	}
	result.container.Details = &domain.ContainerDetails{
		Running:      inspection.Running,
		OOMKilled:    inspection.OOMKilled,
		ExitCode:     inspection.ExitCode,
		RestartCount: inspection.RestartCount,
		StartedAt:    overviewTime(inspection.StartedAt),
		FinishedAt:   overviewTime(inspection.FinishedAt),
	}
	if health := overviewHealth(inspection.Health); health != nil {
		result.container.Health = health
	}
	if inspection.Status != "" {
		result.container.State = overviewText(inspection.Status, 32)
	}
	if inspection.Running {
		stats, statsErr := a.docker.ContainerStats(ctx, item.ID)
		if statsErr != nil || !validOverviewStats(stats) {
			result.partial = true
		} else {
			result.container.Metrics = &stats
		}
	}
	result.container.Assessment = assessOverview(result.container, result.container.Details, result.partial)
	return result
}

func assessOverview(item domain.ContainerOverview, details *domain.ContainerDetails, missing bool) domain.ContainerAssessment {
	if details != nil && details.OOMKilled {
		return domain.ContainerAssessment{Level: "critical", Summary: "Out-of-memory kill recorded."}
	}
	if item.Health != nil && *item.Health == "unhealthy" {
		return domain.ContainerAssessment{Level: "critical", Summary: "Health check reports unhealthy."}
	}
	if details != nil && !details.Running {
		if details.ExitCode != 0 {
			return domain.ContainerAssessment{Level: "warning", Summary: "Stopped with a non-zero exit code."}
		}
		return domain.ContainerAssessment{Level: "warning", Summary: "Not running."}
	}
	if details == nil && !strings.EqualFold(item.State, "running") {
		return domain.ContainerAssessment{Level: "warning", Summary: "Not running; details unavailable."}
	}
	if details != nil && details.RestartCount > 0 {
		return domain.ContainerAssessment{Level: "warning", Summary: "Restarts recorded."}
	}
	if item.Metrics != nil && item.Metrics.MemoryLimit > 0 && item.Metrics.MemoryPercent >= 90 {
		return domain.ContainerAssessment{Level: "warning", Summary: "Memory use is at least 90%."}
	}
	if missing || details == nil {
		return domain.ContainerAssessment{Level: "unknown", Summary: "Details or resources unavailable."}
	}
	if item.Health != nil && *item.Health == "starting" {
		return domain.ContainerAssessment{Level: "unknown", Summary: "Health check is starting."}
	}
	return domain.ContainerAssessment{Level: "ok", Summary: "No warning signals in this snapshot."}
}

func overviewHealth(value *string) *string {
	if value == nil {
		return nil
	}
	health := strings.ToLower(strings.TrimSpace(*value))
	switch health {
	case "healthy", "unhealthy", "starting":
		return &health
	default:
		return nil
	}
}

func overviewTime(value string) string {
	if value == "" || len(value) > 64 {
		return ""
	}
	if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
		return ""
	}
	return overviewText(value, 64)
}

func overviewText(value string, limit int) string {
	var builder strings.Builder
	for _, character := range strings.TrimSpace(value) {
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			continue
		}
		if builder.Len()+len(string(character)) > limit {
			break
		}
		builder.WriteRune(character)
	}
	return builder.String()
}

func validOverviewStats(stats domain.Stats) bool {
	return !math.IsNaN(stats.CPUPercent) && !math.IsInf(stats.CPUPercent, 0) &&
		!math.IsNaN(stats.MemoryPercent) && !math.IsInf(stats.MemoryPercent, 0) &&
		stats.CPUPercent >= 0 && stats.CPUPercent <= 100000 &&
		stats.MemoryPercent >= 0 && stats.MemoryPercent <= 100 &&
		stats.MemoryUsage <= 1<<53-1 && stats.MemoryLimit <= 1<<53-1
}

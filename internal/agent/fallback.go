package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/smn-pascal/dopsy/internal/domain"
)

func (a *Agent) fallback(ctx context.Context, containerID string) (domain.Diagnosis, error) {
	diagnosis := domain.Diagnosis{
		Evidence: make([]domain.Evidence, 0),
		Steps:    make([]domain.Step, 0),
	}
	if containerID == "" {
		containers, err := a.docker.ListContainers(ctx)
		if err != nil {
			return domain.Diagnosis{}, fmt.Errorf("list containers: %w", err)
		}
		diagnosis.Steps = append(diagnosis.Steps, domain.Step{Tool: "list_containers", Summary: fmt.Sprintf("Inspected the container list (%d found)", len(containers))})
		if len(containers) == 0 {
			diagnosis.Answer = "No containers are available to inspect. Connect Docker or start a container, then try again."
			return diagnosis, nil
		}
		containerID = containers[0].ID
	}

	inspection, err := a.docker.InspectContainer(ctx, containerID)
	if err != nil {
		return domain.Diagnosis{}, fmt.Errorf("inspect container: %w", err)
	}
	diagnosis.Steps = append(diagnosis.Steps, domain.Step{Tool: "inspect_container", Summary: "Inspected container state and exit information"})
	diagnosis.Evidence = appendEvidence(diagnosis.Evidence, inspectionEvidence(inspection)...)

	logs, logErr := a.docker.ContainerLogs(ctx, containerID, domain.LogOptions{Tail: min(200, a.maxLogLines)})
	logShowsOOM := false
	if logErr == nil {
		logs.Text, logs.Truncated = truncateUTF8(logs.Text, a.maxLogBytes, logs.Truncated)
		diagnosis.Steps = append(diagnosis.Steps, domain.Step{Tool: "get_container_logs", Summary: "Read recent bounded logs"})
		logSignals := logEvidence(logs)
		logShowsOOM = len(logSignals) > 0
		diagnosis.Evidence = appendEvidence(diagnosis.Evidence, logSignals...)
	}

	if inspection.OOMKilled {
		diagnosis.Answer = "Docker reports OOMKilled=true, so the container was terminated because it ran out of memory. Exit code 137 means the process received SIGKILL. Increase the container memory limit only if the workload genuinely needs it, and inspect the application's memory growth or heap settings before restarting it."
		if logShowsOOM {
			diagnosis.Answer = "Docker reports OOMKilled=true, so the container was terminated because it ran out of memory. The recent logs independently contain an out-of-memory signal, and exit code 137 means the process received SIGKILL. Increase the container memory limit only if the workload genuinely needs it, and inspect the application's memory growth or heap settings before restarting it."
		}
		return diagnosis, nil
	}

	if inspection.ExitCode == 137 {
		diagnosis.Answer = "The container exited with code 137, which means its process received SIGKILL, but Docker did not report OOMKilled=true. The available evidence does not prove why it was killed; check host memory pressure and operator or orchestrator actions."
		if logShowsOOM {
			diagnosis.Answer = "The container exited with code 137 and its recent logs contain an out-of-memory signal, so memory pressure is the likely cause, although Docker did not report OOMKilled=true. Check host memory pressure and the application's memory growth before changing its limit."
		}
		return diagnosis, nil
	}

	state := strings.TrimSpace(inspection.Status)
	if state == "" {
		state = "unknown"
	}
	diagnosis.Answer = fmt.Sprintf("AI analysis is not configured. The read-only inspection completed successfully and the container is currently %s. Configure DOPSY_LLM_MODEL (and provider credentials when required) for a multi-step diagnosis.", state)
	return diagnosis, nil
}

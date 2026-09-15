# Dashboard

The overview dashboard is Dopsy's facts-first starting point. It builds a
current, read-only snapshot of the containers visible to Dopsy and does not
contact the configured AI provider.

## What the snapshot shows

The summary separates four useful signals:

- **Total** is the number of containers returned by Docker.
- **Running** is the number currently reported as running.
- **Healthy** counts containers whose Docker health check reports `healthy`.
- **Needs review** counts deterministic warning or critical assessments in the
  observed subset, not necessarily the entire fleet.

Each observed container can include its current state, health, restart count,
exit code, CPU use, and memory use. These values are point-in-time facts. Dopsy
does not present them as trends and does not infer a root cause from the
dashboard alone.

The snapshot is manual by default. The optional **Live** switch requests a
fresh snapshot at most once per minute while the overview tab is active and
visible. It stops scheduling reads when the browser tab is hidden or the user
opens the diagnostic workspace, and never overlaps an in-progress refresh.
This is repeated point-in-time collection, not stored history; switching Live
on does not turn the charts into trends. The setting resets when the page is
reloaded.

The web dashboard uses a four-number status band, two snapshot charts, and
individual container cards. The CPU columns show up to five observed containers
with current statistics, ranked by current CPU use. Their labelled scale adapts
to the values and can exceed 100% on multi-core workloads. There is no time
axis, history, or change percentage.

The state ring compares **running** with **not running** across the full
container list. The coloured assessment bar describes only the observed subset
and is not a fleet-wide health rate. When the 50-container cap applies, Dopsy
prioritizes conspicuous states for observation, so that subset is not a random
sample. Container cards show available CPU and RAM facts; a RAM percentage bar
appears only when Docker reports a memory limit. Missing readings are shown as
unavailable, never as zero.

German status labels are display text, not additional diagnoses. For a stopped
container, an older health-check result does not override the stopped state on
its card.

Containers without a Docker health check are not labelled unhealthy. An exited
container is marked for review because it is not running, but the dashboard
does not claim that the exit was unexpected. Open an investigation when you
need Dopsy to correlate state, bounded logs, and other available evidence.

## Collection limits

Overview collection is deliberately bounded:

- at most 50 containers receive detail and resource reads in one snapshot;
- no more than four container reads run concurrently;
- each container has a short collection timeout;
- statistics are requested only for running containers;
- logs are never read for the dashboard;
- the model provider is never contacted for the dashboard.

When Docker returns more containers than the observation limit, the response
states how many were observed and marks the snapshot as truncated. Summary
counts for total, running, and healthy describe the full container list, while
needs-review, detail, and resource values cover only the observed subset.

## Partial snapshots

A failure to read one container does not discard the useful facts collected
for the others. The dashboard marks the snapshot as partial near the top and
shows the observed/total coverage. Missing details or metrics remain unavailable
instead of displaying zero.

If the initial container list cannot be read, the overview fails because there
is no reliable fleet snapshot to present. Use the refresh control after fixing
the Docker connection.

## Moving into an investigation

Choose **Diagnose** for a stack-wide question, or use a container's
**Untersuchen** action to open the diagnostic workspace with that container as
the scope. Conversations remain separate from dashboard collection: only a
submitted question can start model-guided tool calls.

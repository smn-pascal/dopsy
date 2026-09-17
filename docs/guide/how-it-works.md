# How it works

Dopsy opens with a bounded overview snapshot. It checks service and Docker
connectivity, loads the current container list, and collects sanitized details
plus current statistics for a limited number of containers. Statistics are
requested only for running containers. The overview does not read logs or
contact the configured model.

```text
page load
  -> health and Docker connectivity check
  -> bounded container list
  -> bounded details and current statistics
  -> deterministic dashboard assessment
```

The [dashboard guide](/guide/dashboard) describes the snapshot semantics,
partial results, and collection limits.

For a diagnosis, the investigation continues through the constrained tool path:

```text
question
  -> provider requests a defined read-only tool
  -> Dopsy validates scope and limits
  -> companion proxy permits an approved Docker read
  -> inspect data is reduced and sanitized
  -> provider receives bounded evidence
  -> answer, evidence, and tool summary
```

The provider never connects to Docker. It can request only the tools implemented
by Dopsy: container listing, sanitized inspection, bounded log reads, retained
container lifecycle/health events, and a non-streaming statistics snapshot.
Agent rounds, tool calls, time windows, bytes,
and request duration all have server-side limits.

Follow-up questions use short-lived context in the Dopsy server process. The
server issues an opaque conversation identifier; the browser returns it only for
the same container scope. Raw tool output is not retained as chat history, and
all in-memory sessions disappear when Dopsy restarts.

### Starting a new diagnosis

On the development branch, **Neue Diagnose** clears the visible chat and any
unsent draft, forgets its conversation identifier, and keeps the selected
container. It sends no request by itself; the next question starts fresh,
without previous conversation context. The control is disabled while a
diagnosis is running, so a reset cannot discard an in-flight answer.

This is a workspace reset, not an immediate server-side deletion. Old context
remains subject to the bounded in-memory store: sessions become unavailable
after 30 minutes of inactivity by default, can be evicted when capacity is
needed, and disappear on server restart. Expired entries are cleaned up when
the next diagnosis begins. No browser persistence or conversation archive is
introduced.

If a follow-up reaches a session that has expired or disappeared, Dopsy keeps
the visible messages, explains that the old context is unavailable, and offers
**Als neue Diagnose senden**. Only clicking that button or submitting a new
question starts another request, without the expired identifier. There is no
automatic provider retry. Retrying an older error retains that question's
original container scope, not a later selection.

The current release does not store historical metrics. Dashboard values are a
point-in-time snapshot, and a diagnosis must state when requested evidence is
unavailable.

## Retained Docker events

The `get_container_events` tool reads a fixed window for a single container.
Without timestamps it reads the past hour, ending one second before the
request. An explicit window requires both `since` and `until` in RFC3339,
must end strictly in the past, and may span at most 24 hours. The operator's
configured timezone is given to the model to interpret relative user dates.

The gateway first resolves a name or short ID through sanitized inspection,
then requests events using that resolved ID. Only `start`, `stop`, `die`,
`oom`, `restart`, `kill`, and known health-status changes are returned. These
are records of past actions, never commands Dopsy can execute. The response
contains only action and Unix-second timestamp; labels, actor attributes,
commands, health-check output, and environment variables are not included.
Events are sorted chronologically within the returned subset.

The gateway and companion proxy each bound the input to 256 KiB and 200
matching records. Excess records set `truncated`; oversized or malformed
responses fail safely rather than exposing partial raw JSON. Requests have a
12-second network deadline and still share the agent's total call/output and
diagnosis budgets. Dashboard collection never requests events.

::: warning NOT A COMPLETE HISTORY
Docker only retains a limited recent event buffer (up to its last 256 events).
Older events can therefore be missing even if the requested time range is
valid. `historyLimited` is always true, including for an empty result. No
matching records does not prove that nothing happened. Dopsy does not persist
events or historical metrics in this version.
:::

See [Docker's event documentation](https://docs.docker.com/reference/cli/docker/system/events/)
for the upstream retention limit.

During a model-guided diagnosis, selected event facts and the history warning
are shown alongside the explanation as evidence. The no-provider demo also
includes fixed sample OOM/exit events when the extra read fits the run budgets;
it never queries a production Docker daemon.

Demo inspection, log excerpts, and event records use the same relative example
times. Demo log reads honor the requested time window and tail limit, so an
unrelated time window does not return the sample crash logs.

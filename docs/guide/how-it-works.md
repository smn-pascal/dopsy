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
by Dopsy: container listing, sanitized inspection, bounded log reads, and a
non-streaming statistics snapshot. Agent rounds, tool calls, log windows, bytes,
and request duration all have server-side limits.

Follow-up questions use short-lived context in the Dopsy server process. The
server issues an opaque conversation identifier; the browser returns it only for
the same container scope. Raw tool output is not retained as chat history, and
all in-memory sessions disappear when Dopsy restarts.

The current release does not store historical metrics. Dashboard values are a
point-in-time snapshot, and a diagnosis must state when requested evidence is
unavailable.

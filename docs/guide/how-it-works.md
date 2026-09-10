# How it works

Dopsy performs two small reads when the interface opens: it checks service and
Docker connectivity, then loads the current container list. It does not inspect
a container, read logs or statistics, or contact the configured model until you
submit a diagnostic question.

```text
page load
  -> health and Docker connectivity check
  -> bounded container list
```

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

The first release does not store historical metrics. A diagnosis must state when
the requested evidence is unavailable.

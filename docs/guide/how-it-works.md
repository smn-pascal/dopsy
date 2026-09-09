# How it works

Dopsy performs analysis only when somebody asks a question.

```text
Question
  -> model chooses a read-only tool
  -> Dopsy validates and limits the request
  -> restricted Docker proxy reads approved data
  -> Dopsy removes sensitive inspect fields
  -> model receives compact evidence
  -> diagnosis and recommendations
```

The model never connects to Docker itself. It can only request the small set of tools implemented and validated by Dopsy. Every diagnosis has hard limits for agent rounds, log lines, bytes, and time.

Follow-up questions use short-lived conversation context stored only in the Dopsy
server process. The browser sends a random conversation identifier, not an editable
copy of earlier messages or tool evidence. Raw tool output is not retained as chat
history, and all in-memory sessions disappear when Dopsy restarts.

The first release does not store historical metrics. A diagnosis must clearly say when the requested historical evidence is unavailable.

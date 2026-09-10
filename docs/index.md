---
layout: home

hero:
  name: Dopsy
  text: Understand your containers.
  tagline: Read-only Docker diagnostics, with the supporting evidence attached.
  actions:
    - theme: brand
      text: Start locally
      link: /guide/getting-started
    - theme: alt
      text: Read the security model
      link: /security/read-only

features:
  - title: Focused reads
    details: Dopsy requests bounded container state, logs, or statistics for the question instead of forwarding an unrestricted data dump.
  - title: Explicit boundary
    details: The diagnostic path has no start, stop, restart, exec, delete, create, or deployment tools.
  - title: Operator-owned provider
    details: Connect an OpenAI-compatible endpoint you control. Dopsy does not run a hosted model service.
---

## Current scope

Dopsy v0.1 covers one path end to end: list containers, inspect sanitized state,
read bounded logs or a live statistics snapshot, and return a diagnosis with the
evidence used. It does not collect historical metrics or modify containers.

## A diagnosis you can inspect

![Dopsy showing an evidence-based out-of-memory diagnosis](/images/dopsy-diagnosis.png)

The screenshot uses deterministic local demo data. In live mode, the same view
lists the read-only operations performed and the resulting container facts.

## Know the boundary

The application container does not mount the Docker socket. A companion proxy
holds it and permits only an exact set of diagnostic reads. This narrows the
exposed API surface; it is not a perfect sandbox. Review the
[read-only design](/security/read-only) before connecting a production host.

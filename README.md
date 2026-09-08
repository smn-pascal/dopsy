# Dopsy

**Understand your containers.**

Dopsy is an AI-powered, self-hosted diagnostic assistant for Docker environments. It combines a conversational interface with targeted, read-only access to container logs, metrics, events, health data, and configuration to help explain what happened and suggest practical next steps.

## Vision

Instead of sending every available log line to an AI model, Dopsy asks for only the data needed to investigate a specific question. A diagnosis can proceed in several focused steps: inspect current container state, examine a relevant time window, correlate events and metrics, and then present an evidence-based explanation.

## Planned V1

- Chat-based diagnostics for Docker containers
- Model-directed, multi-step investigation
- Read-only Docker access
- Focused collection of logs, metrics, events, and container metadata
- Configurable local data retention
- Self-hosted deployment
- Bring your own AI model or API provider

## Safety

The first release is intentionally read-only. Dopsy will diagnose and recommend actions, but it will not restart, stop, delete, or modify containers.

## Status

Dopsy is in early development. Architecture, setup instructions, and contribution guidelines will be added as the first working version takes shape.


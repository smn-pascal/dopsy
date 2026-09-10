<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="brand/dopsy-logo-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="brand/dopsy-logo-light.svg">
    <img alt="Dopsy" src="brand/dopsy-logo-light.svg" width="220">
  </picture>
</p>
<p align="center"><strong>Read-only Docker diagnostics, with the evidence attached.</strong></p>
<p align="center">
  <a href="https://github.com/smn-pascal/dopsy/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/smn-pascal/dopsy/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://smn-pascal.github.io/dopsy/"><img alt="Documentation" src="https://github.com/smn-pascal/dopsy/actions/workflows/docs.yml/badge.svg"></a>
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-64e6bd.svg"></a>
</p>

Dopsy is a self-hosted diagnostic interface for Docker. Ask a question in plain language; Dopsy gathers bounded, read-only evidence from container state, recent logs, and current resource usage, then shows both its diagnosis and the facts behind it.

> [!WARNING]
> Dopsy v0.1 is a development preview and does not include authentication. Keep it bound to localhost; do not expose it directly to the internet.

![Dopsy showing an evidence-based out-of-memory diagnosis](docs/public/images/dopsy-diagnosis.png)

_Deterministic local demo; no Docker daemon, production data, or external model provider._

## Why Dopsy?

Logs tell you what a container printed. Dopsy connects those records to container state and current resource facts so you can investigate why something happened.

```text
"Why does my API keep restarting?"
        ↓
configured model selects from bounded read tools
        ↓
inspect + bounded logs + current stats
        ↓
Exit 137 + OOMKilled + memory pressure
        ↓
diagnosis, next checks, and visible evidence
```

- **Read-only boundary:** no exec, start, stop, restart, delete, create, or deployment operations.
- **Bounded investigations:** rounds, tool calls, log windows, lines, bytes, and request time are capped.
- **Provider choice:** connect an OpenAI-compatible cloud, company, or local endpoint.
- **Evidence stays visible:** deterministic Docker facts are shown alongside the model-generated explanation.

## Architecture

```mermaid
flowchart LR
    Browser[Diagnostic chat] --> App[Dopsy application]
    App -->|tool calls| Model[Your AI provider]
    App -->|approved reads only| Proxy[Restricted Docker proxy]
    Proxy -->|sanitized state, logs, stats| Docker[Docker Engine]
```

The model never receives Docker credentials and cannot call Docker directly. Dopsy validates and bounds each diagnostic request before the companion proxy reaches the Docker API.

## Quick start

```bash
git clone https://github.com/smn-pascal/dopsy.git
cd dopsy
cp .env.example .env
docker compose up --build
```

Open [http://localhost:8080](http://localhost:8080).

For installation details and configuration, read the [live documentation](https://smn-pascal.github.io/dopsy/).

To try the built-in example without an API key or production containers, set this in `.env`:

```dotenv
DOPSY_DEMO_MODE=true
```

To connect a tool-capable OpenAI-compatible endpoint:

```dotenv
DOPSY_LLM_BASE_URL=https://api.example.com/v1
DOPSY_LLM_API_KEY=replace-me
DOPSY_LLM_MODEL=your-model
```

Provider credentials stay on the server and are never returned to the browser.
Bounded log excerpts can still contain secrets. When an external provider is configured,
the excerpts selected during a diagnosis are sent to that provider.

## Security model

The recommended Compose setup gives the Dopsy application **no Docker socket mount**. A separate companion process holds the socket, sanitizes container metadata, and permits only an exact allowlist of diagnostic `GET`/`HEAD` routes and query parameters. Broad endpoints such as container archive/export and every mutating HTTP method are rejected.

This is deliberate: mounting `docker.sock` with `:ro` does not make the Docker API read-only. See [the read-only design](docs/security/read-only.md) for the threat model and current limitations.

## Development

The backend is written in Go. The web interface uses React, TypeScript, and Vite.

```bash
# terminal 1: demo API
make dev-api

# terminal 2: web UI
pnpm install
make dev-web
```

Run the verification suite before committing:

```bash
make test
pnpm typecheck
pnpm docs:build
```

## Project status

Dopsy is pre-1.0 software. The current preview covers one diagnostic path end to end: container discovery, bounded inspection, recent logs, current resource statistics, model-guided tool calls, and visible supporting evidence. Historical metrics, authentication, multi-host operation, alerting, and automated remediation are not implemented.

See the [roadmap](ROADMAP.md), [changelog](CHANGELOG.md), [live documentation](https://smn-pascal.github.io/dopsy/), and [contribution guide](CONTRIBUTING.md).

## License

Dopsy is available under the [MIT License](LICENSE).

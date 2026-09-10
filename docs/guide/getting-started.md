# Getting started

::: warning DEVELOPMENT PREVIEW
Dopsy v0.1 has no authentication. Keep the default localhost binding unless you
put it behind an authenticated TLS reverse proxy.
:::

## Requirements

- Git
- Docker Engine with Docker Compose
- Optional: access to a tool-capable OpenAI-compatible endpoint

## Run the local demo

The demo diagnosis uses fixed container data and does not inspect your running
workloads. It needs no AI provider.

```bash
git clone https://github.com/smn-pascal/dopsy.git
cd dopsy
cp .env.example .env
DOPSY_DEMO_MODE=true docker compose up --build -d
```

Open `http://localhost:8080`, select `demo-api`, and ask why it stopped.

The standard Compose file still starts Dopsy's companion proxy and mounts the
Docker socket into that proxy, even in demo mode. Dopsy itself uses its built-in
sample gateway for the demo and does not request production container data.

::: info Demo mode and providers
With no model configured, demo diagnosis stays local. If `.env` contains
`DOPSY_LLM_MODEL`, Dopsy will contact that provider even in demo mode.
:::

## Connect Docker

Set demo mode to false in `.env`, then start the recommended Compose stack:

```dotenv
DOPSY_DEMO_MODE=false
```

```bash
docker compose up --build -d
```

The application reaches Docker only through the restricted companion proxy in
the Compose network.

## Connect a model

Set these values in `.env` and restart Dopsy:

```dotenv
DOPSY_LLM_BASE_URL=https://api.example.com/v1
DOPSY_LLM_API_KEY=replace-me
DOPSY_LLM_MODEL=your-tool-capable-model
```

The key stays in the server process and is never returned to the browser. An API
key can be left empty for a local endpoint that does not require one.

Set `DOPSY_TIMEZONE` to the host operator's IANA timezone (for example
`Europe/Berlin`) so phrases such as “last night at midnight” are translated into
the correct Docker log window.

Selected, bounded log excerpts are sent to the configured provider and may
contain sensitive application data. Choose the endpoint accordingly.

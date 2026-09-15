# Getting started

::: warning DEVELOPMENT PREVIEW
Dopsy v0.1 has no authentication. Keep the default localhost binding unless you
put it behind an authenticated TLS reverse proxy.
:::

## Requirements

- Docker Engine with Docker Compose
- Docker Compose 2.24.4 or newer for the published-image overlay
- Git only if you want to build from source
- Optional: access to a tool-capable OpenAI-compatible endpoint

## Run the local demo

The demo diagnosis uses fixed container data and does not inspect your running
workloads. It needs no AI provider.

```bash
mkdir dopsy && cd dopsy
curl -fsSLO https://raw.githubusercontent.com/smn-pascal/dopsy/v0.1.3/compose.yaml
curl -fsSLO https://raw.githubusercontent.com/smn-pascal/dopsy/v0.1.3/compose.images.yaml
curl -fsSLo .env https://raw.githubusercontent.com/smn-pascal/dopsy/v0.1.3/.env.example
DOPSY_DEMO_MODE=true docker compose -f compose.yaml -f compose.images.yaml up --pull always --no-build -d
```

Both prebuilt images use the same v0.1.3 tag. The overlay removes the local
build definitions, pulls the tagged images, and retains the base Compose
file's localhost binding and socket isolation. No Git checkout or local
application build is required.

To build the published source yourself instead:

```bash
git clone --depth 1 --branch v0.1.3 https://github.com/smn-pascal/dopsy.git
cd dopsy
cp .env.example .env
DOPSY_DEMO_MODE=true docker compose up --build -d
```

This checks out the published v0.1.3 source rather than the moving `main`
branch. The Compose setup builds the application and companion proxy locally.

Open `http://localhost:8080`. The overview immediately shows the fixed demo
containers and their current resource snapshot without contacting an AI
provider. Choose **Untersuchen** on `demo-api` and ask why it stopped to try the
diagnostic flow.

Both Compose routes still start Dopsy's companion proxy and mount the
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

If you used the published images, run:

```bash
docker compose -f compose.yaml -f compose.images.yaml up --pull always --no-build -d
```

If you chose the source checkout instead, run `docker compose up --build -d`.

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

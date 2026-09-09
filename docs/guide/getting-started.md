# Getting started

> Dopsy is in early development. The commands below build the current version from source.

## Requirements

- Docker Engine with Docker Compose
- An optional API key for an OpenAI-compatible model endpoint

## Start Dopsy

```bash
git clone https://github.com/smn-pascal/dopsy.git
cd dopsy
cp .env.example .env
docker compose up --build -d
```

Open `http://localhost:8080`.

To explore a deterministic example OOM diagnosis without sending data anywhere, add this to `.env`:

```dotenv
DOPSY_DEMO_MODE=true
```

## Connect a model

Set these values in `.env` and restart Dopsy:

```dotenv
DOPSY_LLM_BASE_URL=https://api.example.com/v1
DOPSY_LLM_API_KEY=replace-me
DOPSY_LLM_MODEL=your-tool-capable-model
```

The key stays in the server process. It is never returned to the browser.

Set `DOPSY_TIMEZONE` to the host operator's IANA timezone (for example
`Europe/Berlin`) so phrases such as “last night at midnight” are translated into
the correct Docker log window.

Keep the default localhost port binding unless Dopsy is placed behind an
authenticated TLS reverse proxy. The v0.1 preview does not provide authentication.

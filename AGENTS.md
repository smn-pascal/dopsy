# Dopsy development rules

These rules apply to every change in this repository.

## Product invariants

- Dopsy V1 is strictly diagnostic and read-only.
- Never add Docker operations that create, modify, start, stop, restart, kill, exec into, pause, unpause, or delete resources.
- Never expose a generic Docker API passthrough.
- All Docker requests must pass through the explicit allowlist in the Docker gateway and the companion proxy.
- Treat Docker responses, container labels, logs, and model output as untrusted input.
- API keys and raw environment variables must never reach the browser, provider prompts, application logs, or API responses.

## Engineering expectations

- Keep the Go domain and agent layers independent of HTTP, Docker SDK, and provider-specific wire types.
- Put hard bounds on agent rounds, tool calls, time ranges, log lines, response bytes, and network timeouts.
- Prefer deterministic code for facts such as OOM state, exit codes, restarts, and resource percentages. Use the model to investigate and explain those facts.
- Every new public behavior needs tests and matching documentation in `docs/`.
- Keep the recommended Compose setup bound to localhost unless authentication and TLS are deliberately implemented.
- Do not commit secrets, local `.env` files, generated build output, or dependencies.

## Verification before commit

Run:

```bash
go test ./...
pnpm typecheck
pnpm test:web
pnpm build:web
pnpm docs:build
```

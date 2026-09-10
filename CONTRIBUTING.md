# Contributing to Dopsy

Dopsy is at an early stage, so opening an issue before a large change is encouraged.

## Local development

1. Install a current Go toolchain, Node.js, and pnpm.
2. Run `pnpm install`.
3. Start `make dev-api` and `make dev-web` in separate terminals.
4. Open `http://localhost:5173`.

The API uses safe demo data when started through `make dev-api`, so a Docker daemon and model API key are not required for frontend work.

## Pull requests

- Keep changes focused and explain the user-visible outcome.
- Add tests for behavior and security boundaries.
- Update `docs/` with every public feature or configuration change.
- Never weaken the read-only Docker allowlist to make a feature easier.
- Never include real logs, API keys, environment files, or other secrets in fixtures.
- Use a short, imperative commit subject and keep unrelated changes separate.

Run the full verification suite before requesting review:

```bash
go test ./...
go vet ./...
pnpm typecheck
pnpm test:web
pnpm build:web
pnpm docs:build
```

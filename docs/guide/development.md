# Local development

Use a source checkout of `main` to work on features that are not yet in a
published release. Install Go, Node.js, and pnpm, then run `pnpm install` in the
repository root.

Start the API and web interface in separate terminals:

```bash
make dev-api
```

```bash
make dev-web
```

Open `http://localhost:5173` (or `http://127.0.0.1:5173` if the local server
listens there). `make dev-api` uses demo container data and needs no Docker
daemon. Leave `DOPSY_LLM_MODEL` unset for a deterministic diagnosis without an
external provider. If a model is configured, demo evidence can still be sent
to that provider.

## API forwarding and origin protection

The development web server forwards `/api` to `http://127.0.0.1:3001` while
preserving the browser-facing `Host` and `Origin`. Changing the host to the
upstream API address makes chat POST requests fail with
`cross-origin request rejected`. Do not work around this by removing the
origin check or replacing untrusted origin headers.

This forwarding applies only to development. The production server serves the
web interface and API together; its cross-origin protection is unchanged.
Keep both development services local; this is not a public hosting setup.

## Before committing

Run the repository's full verification suite:

```bash
go test ./...
pnpm typecheck
pnpm test:web
pnpm build:web
pnpm docs:build
```

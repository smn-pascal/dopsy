## Summary

Describe the user-visible outcome and why the change belongs in Dopsy.

## Validation

- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `pnpm typecheck`
- [ ] `pnpm test:web`
- [ ] `pnpm build:web`
- [ ] `pnpm docs:build`

## Safety and documentation

- [ ] The change preserves Dopsy's read-only boundary.
- [ ] Tests cover changed behavior and security-sensitive paths.
- [ ] Public behavior or configuration changes are documented.
- [ ] Fixtures, screenshots, and logs contain no credentials or private data.

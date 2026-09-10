# Changelog

All notable changes to Dopsy are documented in this file. The project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-09-10

### Added

- A responsive diagnostic chat for questions about Docker containers.
- Bounded, read-only tools for container listing, inspection, recent logs, and
  current resource statistics.
- An iterative OpenAI-compatible tool-calling loop with operator-supplied model
  credentials.
- A deterministic local demo that needs neither Docker nor an external AI
  provider.
- Short-lived, size-limited server-side conversation context for follow-up
  questions.
- A dedicated Docker socket proxy with exact route, method, and query
  allowlists plus sanitized responses.
- A hardened Docker Compose setup, API specification, automated tests, and
  VitePress documentation.

### Security

- The application container has no direct Docker socket access.
- Mutating Docker methods and broad data-export routes are rejected by the
  companion proxy.
- Agent rounds, tool calls, log windows, output sizes, request time, and
  concurrent diagnoses are bounded.

[Unreleased]: https://github.com/smn-pascal/dopsy/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/smn-pascal/dopsy/releases/tag/v0.1.0

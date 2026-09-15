# Changelog

All notable changes to Dopsy are documented in this file. The project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- An opt-in, visible-tab-only dashboard refresh that reads at most once per
  minute and does not overlap an existing snapshot request.

## [0.1.3] - 2026-09-15

### Added

- Versioned Linux amd64 and arm64 images for the application and restricted
  Docker proxy, published to GitHub Container Registry after release checks.
- A pull-only Compose overlay that keeps the existing localhost, socket, and
  read-only boundaries while removing the local build context.
- A release smoke test that starts the published images and runs a demo
  diagnosis before creating the GitHub release.

## [0.1.2] - 2026-09-15

### Added

- A bounded live overview for deterministic container triage before starting
  an AI-assisted investigation.
- Current CPU and memory snapshots for running containers, with explicit
  partial and truncated collection states.

### Changed

- The web interface now opens on the overview and keeps the diagnostic chat as
  a separate investigation workspace. Interface copy is shorter and in German.
- The overview now uses snapshot charts and individual container cards instead
  of a container table, without presenting uncollected historical trends.
- The release process runs the full test and Linux Compose demo checks before a
  tagged version can be published.

## [0.1.1] - 2026-09-10

### Changed

- Reworked the diagnostic interface around a cleaner light workspace,
  restrained forest-green navigation, and clearer information hierarchy.
- Refined the Dopsy mark and wordmark for more balanced spacing at app,
  documentation, and favicon sizes.
- Made supporting evidence and investigation progress easier to scan without
  changing the read-only diagnostic boundary.
- Applied the visual identity consistently across the application,
  documentation, README, favicons, and social-preview artwork.
- Clarified setup, provider data flow, demo behavior, and the limitations of
  the v0.1 preview.

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

[Unreleased]: https://github.com/smn-pascal/dopsy/compare/v0.1.3...HEAD
[0.1.3]: https://github.com/smn-pascal/dopsy/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/smn-pascal/dopsy/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/smn-pascal/dopsy/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/smn-pascal/dopsy/releases/tag/v0.1.0

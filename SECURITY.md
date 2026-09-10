# Security policy

## Development status

Dopsy is not yet production-ready. Security fixes for the latest `0.1.x`
development preview are handled on a best-effort basis; older previews are not
supported.

## Reporting a vulnerability

Please do not open a public issue containing vulnerability details. Use the
repository's private vulnerability reporting form. If that option is not
available, open a minimal issue asking the maintainer for a private contact
channel without including sensitive details.

Include the affected component, reproduction steps, possible impact, and any suggested mitigation. Do not include real credentials or private production logs.

## Security boundaries

Dopsy's read-only promise is a core security invariant. Changes involving the Docker proxy, Docker routes, model tools, log handling, secret filtering, authentication, or browser exposure require dedicated security tests and documentation.

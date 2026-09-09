# Security policy

## Development status

Dopsy is not yet production-ready. No released version currently receives long-term security support.

## Reporting a vulnerability

Please do not open a public issue for a suspected vulnerability. Use GitHub's private vulnerability reporting for this repository when available.

Include the affected component, reproduction steps, possible impact, and any suggested mitigation. Do not include real credentials or private production logs.

## Security boundaries

Dopsy's read-only promise is a core security invariant. Changes involving the Docker proxy, Docker routes, model tools, log handling, secret filtering, authentication, or browser exposure require dedicated security tests and documentation.

# Read-only design

Dopsy is an observer, not a container manager.

## The agent cannot mutate Docker

The model is offered only narrowly defined inspection tools. Dopsy contains no agent tools for executing commands or starting, stopping, restarting, deleting, creating, or modifying containers.

## The Docker socket needs a real boundary

Mounting `/var/run/docker.sock` with `:ro` only makes the socket file mount read-only; it does not turn the Docker API into a read-only API. Dopsy therefore does not receive the socket in the recommended Compose setup.

Instead, a small companion proxy owns the socket and accepts only an exact allowlist of HTTP `GET`/`HEAD` routes and query parameters needed for diagnostics. Broad container endpoints such as filesystem export, archive access, process listings, and every mutating method are denied. Container-list and inspect responses are reduced to the diagnostic fields Dopsy needs. The proxy is reachable only on an internal Docker network.

This meaningfully reduces the exposed API surface, but no socket proxy should be treated as a perfect sandbox. Keep Dopsy bound to localhost unless you place it behind authentication and a correctly configured TLS reverse proxy.

## Data sent to a model

Inspect results exclude environment variables, host mount paths, command arguments, and secret-like labels. Logs may still contain sensitive data. Review your chosen provider and avoid exposing Dopsy directly to untrusted users.

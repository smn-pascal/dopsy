# Read-only design

Dopsy inspects containers. It does not manage them.

## The agent cannot mutate Docker

The provider can request only narrowly defined inspection tools. Dopsy contains
no diagnostic tools for executing commands or starting, stopping, restarting,
deleting, creating, or modifying containers.

## The Docker socket needs a real boundary

Mounting `/var/run/docker.sock` with `:ro` only makes the socket file mount read-only; it does not turn the Docker API into a read-only API. Dopsy therefore does not receive the socket in the recommended Compose setup.

Instead, a small companion proxy owns the socket and accepts only an exact allowlist of HTTP `GET`/`HEAD` routes and query parameters needed for diagnostics. Broad container endpoints such as filesystem export, archive access, process listings, and every mutating method are denied. Container-list and inspect responses are reduced to the diagnostic fields Dopsy needs. The proxy is reachable only on an internal Docker network.

::: danger Treat Docker access as privileged
The proxy substantially narrows the exposed API surface, but it is not a perfect
sandbox. Keep Dopsy bound to localhost unless it is behind authentication and a
correctly configured TLS reverse proxy.
:::

## Data sent to a model

Inspect results exclude environment variables, host mount paths, command
arguments, and secret-like labels. Logs may still contain sensitive data, and
bounded excerpts are sent to the configured provider during analysis. Review the
provider's data handling and do not expose Dopsy directly to untrusted users.

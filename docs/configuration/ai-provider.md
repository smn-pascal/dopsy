# AI provider

Dopsy uses the OpenAI-compatible Chat Completions format with function calling.
The endpoint can be a cloud gateway, a company-managed service, or a local model
server.

| Variable             | Purpose                                        |
| -------------------- | ---------------------------------------------- |
| `DOPSY_LLM_BASE_URL` | Provider API root, usually ending in `/v1`     |
| `DOPSY_LLM_API_KEY`  | Server-side API credential                     |
| `DOPSY_LLM_MODEL`    | Tool-capable model identifier                  |
| `DOPSY_TIMEZONE`     | IANA timezone used to interpret relative times |

No provider is contacted until a user submits a diagnostic request. Setting only
demo mode does not override provider configuration: when `DOPSY_LLM_MODEL` is
set, demo evidence is still sent to the configured endpoint.

The model must support Chat Completions function/tool calling. Dopsy never sends
the API key to the browser. It does send selected, bounded log excerpts to the
provider; application logs may contain secrets. Use an endpoint and retention
policy appropriate for the workload.

## Local endpoint without a key

`DOPSY_LLM_API_KEY` may remain empty when the endpoint does not require bearer
authentication. `DOPSY_LLM_BASE_URL` must be an absolute `http` or `https` URL.

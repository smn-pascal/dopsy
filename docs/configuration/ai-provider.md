# AI provider

Dopsy's first provider adapter targets the widely supported OpenAI-compatible Chat Completions format with function calling. This covers many cloud gateways, company-managed endpoints, and local model servers.

| Variable             | Purpose                                        |
| -------------------- | ---------------------------------------------- |
| `DOPSY_LLM_BASE_URL` | Provider API root, usually ending in `/v1`     |
| `DOPSY_LLM_API_KEY`  | Server-side API credential                     |
| `DOPSY_LLM_MODEL`    | Tool-capable model identifier                  |
| `DOPSY_TIMEZONE`     | IANA timezone used to interpret relative times |

No provider is contacted until a user starts a diagnosis. The operator remains responsible for deciding where container data may be processed.

The configured model must support Chat Completions function/tool calling. Dopsy
never sends the API key to the browser, but selected log excerpts are sent to the
provider and may contain application secrets. Use a provider and retention policy
appropriate for the workload.

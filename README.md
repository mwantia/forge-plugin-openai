# forge-plugin-openai

Provider plugin that connects Forge to OpenAI and any OpenAI-compatible LLM API.

## Capabilities

| Capability | Supported |
|---|---|
| Chat completions | yes |
| Streaming | yes |
| Tool / function calling | yes |
| Embeddings | yes |
| List models | yes |
| Vision | no |
| Model CRUD | no |

## Configuration

```hcl
plugin "provider" "openai" {
  config {
    address = "https://api.openai.com"  # base URL — no /v1 suffix
    token   = ""                         # API key (required)

    http {
      timeout = "60s"
    }
  }
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `address` | string | `https://api.openai.com` | Base URL of an OpenAI-compatible endpoint |
| `token` | string | — | Bearer token / API key |
| `seed` | int | — | Optional fixed seed for deterministic outputs |
| `http.timeout` | duration | `60s` | Timeout for non-streaming calls (probe, embed, list models); streaming calls are controlled by context cancellation |

## Usage

### Chat

```hcl
# Dispatch to a specific model
POST /v1/pipeline/dispatch
{
  "model": "openai/gpt-4o",
  "messages": [...]
}
```

Or define an alias in the provider block:

```hcl
provider {
  model "assistant" {
    base_model = "openai/gpt-4o"
    system     = "You are a helpful assistant."
    options    { temperature = 0.7 }
  }
}
```

Then dispatch to `forge/assistant`.

### Embeddings

Forge calls embeddings internally via `ResourcePlugin` recall. You can also target them directly through the pipeline with an embedding-capable model (e.g. `openai/text-embedding-3-small`).

## Token usage & cost tracking

When model metadata includes pricing fields (`CostPerInputToken`, `CostPerOutputToken`, `CostPerCachedInputToken`), the plugin tracks token costs per turn and returns a breakdown in the final stream chunk. Cached prompt tokens (OpenAI prompt caching) are accounted separately at the cached rate.

## OpenAI-compatible endpoints

Set `address` to any OpenAI-compatible base URL to use third-party providers:

```hcl
# Azure OpenAI
config {
  address = "https://<resource>.openai.azure.com/openai/deployments/<deployment>"
  token   = "<azure-api-key>"
}

# Local (e.g. LM Studio, vLLM)
config {
  address = "http://127.0.0.1:1234"
  token   = "unused"
}
```

## Build

```bash
task build   # outputs ./build/openai
```

Place the binary in the `plugin_dir` configured for your Forge instance.

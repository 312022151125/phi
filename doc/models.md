# Supported models

phi talks to LLMs through an **explicit** `api` field on each config entry
(`OpenAI` | `Anthropic` | `Gemini`). Empty `api` means OpenAI-compatible
`/chat/completions`. There is no name- or URL-based provider guessing.

Built-in **presets** (exact `name` match) fill `base_url`, `context_window`,
`image_enabled`, `api`, and default thinking when those fields are omitted.
Source of truth: [`internal/project/model`](../internal/project/model/).

## Built-in presets

| Name | API | Base URL (default) | Context | Images | Thinking wire |
| ---- | --- | ------------------ | ------: | :----: | ------------- |
| `deepseek-flash` | OpenAI | `https://api.deepseek.com` | 1M | yes | `extra_body.thinking` + `reasoning_effort` |
| `deepseek-v4-pro` | OpenAI | `https://api.deepseek.com` | 1M | no | same as Flash |
| `gemini-2.5-pro` | Gemini | Google Generative Language | 1M | yes | `thinkingBudget` (token cap) |
| `gemini-2.5-flash` | Gemini | Google Generative Language | 1M | yes | `thinkingBudget` |
| `gemini-3-pro` | Gemini | Google Generative Language | 1M | yes | `thinkingLevel` (off floor `LOW`) |
| `gemini-3-flash` | Gemini | Google Generative Language | 1M | yes | `thinkingLevel` (off floor `MINIMAL`) |

Minimal config for a preset (api key only):

```yaml
models:
  - name: deepseek-flash
    api_key: sk-...
    default: true
```

## Other / custom models

Any other `name` works as a normal entry. Set fields yourself:

```yaml
models:
  - name: gpt-4o
    api: OpenAI                 # optional; default
    api_key: sk-...
    base_url: https://api.openai.com/v1
    context_window: 128000
    default: true

  - name: claude-sonnet-4-20250514
    api: Anthropic              # required for Anthropic Messages API
    api_key: sk-ant-...
    base_url: https://api.anthropic.com
    context_window: 200000

  - name: my-proxy-model
    api: OpenAI
    api_key: ...
    base_url: https://proxy.example/v1
```

Gemini without a preset name still needs `api: Gemini` and a valid base URL;
thinking then uses the budget style by default.

## Thinking level

Provider-agnostic knobs on each model (and session UI):

| Key / env | Meaning |
| --------- | ------- |
| `think_enabled` | bool; enable reasoning payload |
| `think_level` | `off` \| `minimal` \| `low` \| `medium` \| `high` \| `xhigh` \| `max` |
| `PHI_THINK_LEVEL` | env override on the default model (`off` disables) |

How that maps on the wire depends on the provider / preset interceptor
(OpenAI `reasoning_effort`, Anthropic thinking budget, Gemini budget or level).

```yaml
models:
  - name: gemini-2.5-flash
    api_key: ...
    think_level: medium
```

## Related

- Config overview: [README § Configuration](../README.md#configuration)
- Preset code: `internal/project/model/`

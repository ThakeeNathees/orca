# model

The `model` block configures an LLM provider and model.

## Syntax

```orca
model <name> {
  provider    = <string>
  model_name  = <string>
  api_key     = <string>  // optional
  base_url    = <string>  // optional
  temperature = <number>  // optional
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provider` | `string` | Yes | LLM provider: `"openai"`, `"anthropic"`, or `"google"` |
| `model_name` | `string \| model` | Yes | The model identifier (e.g., `"gpt-4o"`, `"claude-sonnet"`) |
| `api_key` | `string \| nulltype` | No | API key for the provider (overrides environment variable) |
| `base_url` | `string \| nulltype` | No | Custom base URL for the provider endpoint |
| `temperature` | `number \| nulltype` | No | Sampling temperature (0.0 – 1.0) |

## Supported providers

| Provider | Generated class | Python package |
|----------|----------------|----------------|
| `"openai"` | `ChatOpenAI` | `langchain-openai` |
| `"anthropic"` | `ChatAnthropic` | `langchain-anthropic` |
| `"google"` | `ChatGoogleGenerativeAI` | `langchain-google-genai` |

The compiler automatically adds the correct package to `pyproject.toml` based on which providers you use.

## Examples

### OpenAI

```orca
model gpt4 {
  provider    = "openai"
  model_name  = "gpt-4o"
  temperature = 0.7
}
```

### Anthropic

```orca
model claude {
  provider    = "anthropic"
  model_name  = "claude-sonnet-4-20250514"
  temperature = 0.5
}
```

### Google

```orca
model gemini {
  provider    = "google"
  model_name  = "gemini-2.0-flash"
}
```

## Generated output

```orca
model gpt4 {
  provider    = "openai"
  model_name  = "gpt-4o"
  temperature = 0.7
}
```

Compiles to:

```python
from langchain_openai import ChatOpenAI

gpt4 = ChatOpenAI(model="gpt-4o", temperature=0.7)  # main.orca:1
```

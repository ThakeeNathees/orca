# agent

The `agent` block defines an AI agent with a model, persona, and optional tools.

## Syntax

```orca
agent <name> {
  model         = <model_ref>
  persona       = <string>
  tools         = [<tool_ref>, ...]  // optional
  output_schema = <schema_ref>       // optional
  temperature   = <number>           // optional
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `model` | `string \| model` | Yes | Reference to a model block or a model string |
| `persona` | `string` | Yes | The agent's system prompt / behavior description |
| `tools` | `list[tool] \| nulltype` | No | List of tool references the agent can use |
| `output_schema` | `schema \| nulltype` | No | Structured output schema for the agent's response |
| `temperature` | `number \| nulltype` | No | Sampling temperature override. Takes precedence over the `temperature` set on the referenced `model` block |

## Examples

### Basic agent

```orca
model gpt4 {
  provider    = "openai"
  model_name  = "gpt-4o"
  temperature = 0.7
}

agent writer {
  model   = gpt4
  persona = "You are a helpful writer. You write clear, concise content."
}
```

### Agent with tools

```orca
tool search {
  name = "web_search"
  desc = "Search the web for information"
}

tool calculator {
  name = "calculator"
  desc = "Perform mathematical calculations"
}

agent researcher {
  model   = gpt4
  persona = "You are a research assistant. Find information and verify facts."
  tools   = [search, calculator]
}
```

### Agent with structured output

```orca
schema report {
  title   = string
  summary = string
  sources = list[string]
}

agent analyst {
  model         = gpt4
  persona       = "You analyze data and produce structured reports."
  output_schema = report
}
```

### Per-agent temperature override

A `model` block sets a default sampling temperature. An `agent` referencing
that model can override it for itself — useful when the same model is reused
at different temperatures for different tasks:

```orca
model gpt4 {
  provider    = "openai"
  model_name  = "gpt-4o"
  temperature = 0.7
}

agent classifier {
  model       = gpt4
  persona     = "You are a precise classifier."
  temperature = 0.0  // overrides gpt4's 0.7 for this agent
}

agent writer {
  model   = gpt4
  persona = "You are a creative writer."
  // inherits temperature = 0.7 from gpt4
}
```

### Multi-line persona

~~~orca
agent researcher {
  model   = gpt4
  persona = ```md
    You are a research assistant.
    You search the web for information.

    Always cite your sources.
    Follow APA citation format.
    ```
}
~~~

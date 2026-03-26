# agent

The `agent` block defines an AI agent with a model, persona, and optional tools.

## Syntax

```orca
agent <name> {
  model   = <model_ref>
  persona = <string>
  tools   = [<tool_ref>, ...]  // optional
  output  = <schema_ref>       // optional
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `model` | `str \| model` | Yes | Reference to a model block or a model string |
| `persona` | `str` | Yes | The agent's system prompt / behavior description |
| `tools` | `list[tool] \| null` | No | List of tool references the agent can use |
| `output` | `schema \| null` | No | Structured output schema for the agent's response |

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
  title   = str
  summary = str
  sources = list[str]
}

agent analyst {
  model   = gpt4
  persona = "You analyze data and produce structured reports."
  output  = report
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

# workflow

The `workflow` block orchestrates multiple agents into a directed graph.

## Syntax

```orca
workflow <name> {
  name = <string>  // optional
  desc = <string>  // optional
  flow = <agent_ref> -> <agent_ref> -> <agent_ref>
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `str \| null` | No | Display name for the workflow |
| `desc` | `str \| null` | No | A description of what the workflow does |

## Arrow syntax

The `flow` field uses the `->` arrow operator to chain agents. Because `flow` is not yet part of the
formal workflow schema, the compiler emits an `unknown-field` warning — suppress it with `@suppress("unknown-field")`.

```orca
@suppress("unknown-field")
workflow pipeline {
  flow = researcher -> writer -> reviewer
}
```

This creates a sequential pipeline where:
1. `researcher` runs first
2. Its output passes to `writer`
3. `writer`'s output passes to `reviewer`

## Examples

### Sequential pipeline

```orca
model gpt4 {
  provider    = "openai"
  model_name  = "gpt-4o"
  temperature = 0.7
}

agent researcher {
  model   = gpt4
  persona = "You research topics thoroughly."
}

agent writer {
  model   = gpt4
  persona = "You write clear, engaging content."
}

agent editor {
  model   = gpt4
  persona = "You edit and polish written content."
}

@suppress("unknown-field")
workflow content_pipeline {
  flow = researcher -> writer -> editor
}
```

::: info
Workflow code generation is under active development. The arrow syntax is parsed and validated, and more complex routing patterns (conditional edges, parallel branches) are planned.
:::

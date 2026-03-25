# workflow

The `workflow` block orchestrates multiple agents into a directed graph.

## Syntax

```orca
workflow <name> {
  flow = <agent_ref> -> <agent_ref> -> <agent_ref>
}
```

## Arrow syntax

Use the `->` operator to define the flow between agents:

```orca
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

workflow content_pipeline {
  flow = researcher -> writer -> editor
}
```

::: info
Workflow code generation is under active development. The arrow syntax is parsed and validated, and more complex routing patterns (conditional edges, parallel branches) are planned.
:::

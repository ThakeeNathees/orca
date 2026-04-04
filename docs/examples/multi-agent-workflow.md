# Multi-Agent Workflow

A complete example with multiple agents orchestrated through a workflow pipeline.

## Source

### `models.oc`

```orca
model gpt4 {
  provider    = "openai"
  model_name  = "gpt-4o"
  temperature = 0.7
}

model claude {
  provider    = "anthropic"
  model_name  = "claude-sonnet-4-20250514"
  temperature = 0.5
}
```

### `agents.oc`

```orca
tool search {
  desc   = "Search the web for information"
  invoke = "tools.search.web_search"
}

agent researcher {
  model   = gpt4
  persona = "
    You are a research specialist.
    You search for information and compile
    detailed findings with sources.
    "
  tools   = [search]
}

agent writer {
  model   = claude
  persona = "
    You are a technical writer.
    You take research findings and turn them
    into clear, well-structured articles.
    "
}

agent editor {
  model   = gpt4
  persona = "
    You are an editor.
    You review articles for clarity, accuracy,
    and grammar. You suggest improvements.
    "
}
```

### `workflow.oc`

```orca
workflow content_pipeline {
  researcher -> writer -> editor
}
```

## Build

```bash
orca build
```

## What's happening

1. Two models are configured — GPT-4o for research/editing, Claude for writing.
2. Three agents form a pipeline: researcher finds information, writer drafts the article, editor polishes it.
3. The `workflow` block connects them with arrow syntax: `researcher -> writer -> editor`.
4. `START` and `END` are inferred automatically — `researcher` has no incoming edges so it becomes the entry point, `editor` has no outgoing edges so it becomes the exit.

This pattern — splitting definitions across multiple `.oc` files — is idiomatic. The compiler reads all `.oc` files in the directory and resolves cross-file references automatically.

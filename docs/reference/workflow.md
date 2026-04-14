# workflow

The `workflow` block orchestrates multiple agents and tools into a directed graph. It compiles to a LangGraph `StateGraph`.

## Syntax

```orca
workflow <name> {
  name = <string>  // optional
  desc = <string>  // optional
  <node> -> <node> -> <node>
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string \| nulltype` | No | Display name for the workflow |
| `desc` | `string \| nulltype` | No | A description of what the workflow does |

## Arrow syntax

The `->` arrow operator chains nodes as bare expressions at block level:

```orca
workflow pipeline {
  researcher -> writer -> reviewer
}
```

The `->` operator binds **tighter** than the ternary `?:` in ordinary expressions (so `a -> b ? c : d` is `(a -> b) ? c : d`). See [Syntax overview — Ternary conditional expressions](/reference/syntax-overview#ternary-conditional-expressions).

This creates a sequential pipeline where:
1. `researcher` runs first
2. Its output passes to `writer`
3. `writer`'s output passes to `reviewer`

## Entry and exit points

The compiler automatically infers graph entry and exit points from the topology:

- **Entry:** Any node with no incoming edges becomes an entry point. [`cron`](/reference/cron) and [`webhook`](/reference/webhook) blocks are valid workflow nodes when you need an explicit schedule- or HTTP-driven entry in the graph (runtime wiring is separate from compilation).
- **Exit:** Any node with no outgoing edges becomes an exit point.

You never need to specify entry/exit explicitly — just define the edges between your nodes.

## Workflow nodes

The following block types can appear as nodes in a workflow:

| Block type | Description |
|-----------|-------------|
| `agent` | An LLM agent that processes input and produces output |
| `tool` | A standalone tool execution (not an agent tool call) |
| `cron` | Cron schedule metadata; use the block’s name as a graph node |
| `webhook` | Webhook path/method metadata; use the block’s name as a graph node |

When a tool appears in a workflow edge, it runs as an independent graph node. This is different from listing a tool in an agent's `tools` field, where it becomes available for the agent to call.

## Multiple edge chains

A workflow can contain multiple edge expressions to define complex graph topologies:

```orca
workflow pipeline {
  researcher -> writer
  writer -> reviewer
  writer -> fact_checker
}
```

Each line is a separate edge chain. Nodes referenced across chains are deduplicated — `writer` appears once in the generated graph.

In this example, `researcher` has no incoming edges so it becomes the entry point. Both `reviewer` and `fact_checker` have no outgoing edges, so both become exit points.

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
  researcher -> writer -> editor
}
```

### Mixed agent and tool nodes

```orca
agent drafter {
  model   = gpt4
  persona = "You draft reports."
}

tool validate {
  desc   = "Validate report against style guide"
  invoke = "tools.validation.check_style"
}

workflow review_pipeline {
  drafter -> validate
}
```

### Fan-out topology

```orca
workflow analysis {
  classifier -> writer
  classifier -> analyst
}
```

Both `writer` and `analyst` run after `classifier`, forming parallel branches.

## Generated code

A workflow block compiles to LangGraph `StateGraph` construction code:

```python
from langgraph.graph import StateGraph, START, END

# Node wrapper functions
def _node_researcher(state: GraphState) -> dict:
    """Workflow node wrapping 'researcher'."""
    pass  # TODO: implement node invocation

def _node_writer(state: GraphState) -> dict:
    """Workflow node wrapping 'writer'."""
    pass  # TODO: implement node invocation

# Build and compile the graph
pipeline = StateGraph(GraphState)
pipeline.add_node("researcher", _node_researcher)
pipeline.add_node("writer", _node_writer)
pipeline.add_edge(START, "researcher")
pipeline.add_edge("researcher", "writer")
pipeline.add_edge("writer", END)
pipeline = pipeline.compile()
```

::: tip
Node wrapper function bodies are currently stubs. Actual agent/tool invocation will be implemented in a future release.
:::

## Planned: Conditional branching

Conditional routing will use a `branch` block inside workflows:

```orca
workflow pipeline {
  classifier -> branch {
    if ctx.category == "technical": tech_writer
    if ctx.category == "creative": creative_writer
  }
}
```

This will map to LangGraph's `add_conditional_edges()`. This feature is not yet implemented.

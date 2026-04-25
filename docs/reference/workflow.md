# workflow

The `workflow` block orchestrates multiple agents and tools into a directed graph. It compiles to a LangGraph `StateGraph`.

## Syntax

```orca
workflow <name> {
  name = <string>   // optional
  desc = <string>   // optional
  nodes = { ... }   // optional — explicit graph node names / duplicates
  <node> -> <node> -> <node>
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string \| nulltype` | No | Display name for the workflow |
| `desc` | `string \| nulltype` | No | A description of what the workflow does |
| `nodes` | `map[string, workflow_node] \| nulltype` | No | Optional registry mapping **graph node id** (string key) to the block that implements that node — see [Explicit `nodes` map](#explicit-nodes-map) |

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
| `branch` | Conditional routing via a constant `route` map |

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

## Explicit `nodes` map

By default, every block you reference in an edge chain is added to the graph under its **block name** (the identifier you declare). That is enough when each physical node appears at most once.

When you need the **same** block to appear as **two different graph nodes** (for example, revisit an agent later in the graph), or you want an edge to use a **string id** that is not the block’s name, declare a `nodes` map. Keys are string literals (graph node ids); values are workflow-capable blocks (`agent`, `tool`, `cron`, `webhook`, inline or named `branch`, and so on).

After registration, you can refer to that id as a **string literal** in edges (and in `branch.route` values) instead of using the block identifier:

```orca
agent A { model = gpt4 }
agent B { model = gpt4 }
model gpt4 { provider = "openai" }

workflow run {
  nodes = {
    "second_A": A   // second graph node, same implementation as A
  }
  A -> B -> "second_A"
}
```

Rules enforced by the analyzer:

- `nodes` must be a **compile-time constant** map (literal or foldable to one). Keys must be constant strings.
- A string used as a workflow node (`"second_A"`) must appear as a key in `nodes` for that workflow. Otherwise you get an error (unknown graph node id).
- Values in `nodes` must each resolve to a valid `@workflow_node` block, same as a non-string node reference.

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

## Conditional branching (`branch`)

Use a `branch` block with a constant `route` map (see [Types — Annotated types](/reference/syntax-overview#annotated-types) for `annotated["workflow_node"]`). Route targets may be block references or **strings** that name a key from the workflow’s `nodes` map (same rules as string literals in edges).

```orca
agent A { model = gpt4 }
agent B { model = gpt4 }
model gpt4 { provider = "openai" }

workflow pipeline {
  nodes = {
    "alt": B
  }
  classifier -> branch {
    route = {
      "route_1": A,
      "route_2": "alt",   // same as B, registered under "alt"
    }
  }
}
```

Codegen maps this to LangGraph conditional edges; `transform` on `branch` is optional for shaping the routing input.

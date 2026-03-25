# Workflow & Agent Orchestration Proposal

## Overview

Workflows are the core orchestration primitive in Orca. A workflow defines how agents connect into an execution graph. Under the hood, a workflow compiles to a LangGraph `StateGraph`.

## Agent output — structured vs unstructured

An agent's `output` field controls how it communicates with downstream nodes. This is the single most important design decision for data flow.

### Unstructured (default)

Agents without `output` are conversational — they read and append to a shared message history. Use this for chatbot-style flows where context should carry through.

```hcl
agent greeter {
  model   = gpt4
  persona = "Greet the user and ask what they need"
}

agent billing_agent {
  model   = gpt4
  persona = "Handle billing questions"
}
```

### Structured

Agents with `output` produce typed fields. Their internal message history is **isolated** — downstream agents receive only the structured data, not the upstream agent's reasoning, tool calls, or retries.

```hcl
agent researcher {
  model   = gpt4
  persona = "Research the topic thoroughly"
  tools   = [web_search]
  output  = schema {
    findings = str
    sources  = list[str]
  }
}

agent classifier {
  model  = gpt4
  persona = "Classify the support ticket"
  output = schema {
    category   = str
    confidence = float
    summary    = str
  }
}
```

The `output` field accepts either an inline `schema { ... }` or a reference to a named schema block:

```hcl
schema report_output {
  draft    = str
  revision = int
}

agent writer {
  model   = gpt4
  persona = "Write a report"
  output  = report_output
}
```

### Why output determines isolation

The `output` block acts as a boundary:

- **Has `output`** → isolated. The agent runs with its own message thread. Only the structured fields cross into the workflow state. Downstream agents never see upstream tool calls, retries, or internal reasoning.
- **No `output`** → shared messages. The agent appends to the workflow's `messages` list. Downstream agents see the full conversation history.

This is not configurable — it follows from whether `output` is defined. Structured agents are isolated; conversational agents share context.

## Workflow syntax

### Linear flow

```hcl
workflow report {
  researcher("Research quantum computing") -> writer("Write the report")
}
```

The `("prompt")` syntax on workflow nodes provides the task-specific instruction for each agent invocation. The agent's `persona` is the system prompt; the workflow prompt is the human message.

Compiles to:
```
START -> researcher -> writer -> END
```

### Branching (conditional edges)

```hcl
workflow support {
  classifier("Classify this ticket") -> {
    urgent:  escalator("Handle urgent case") -> human
    normal:  auto_reply("Send standard response")
    spam:    archive("Archive this ticket")
  }
}
```

The classifier agent must have a structured `output` with a field whose values match the branch keys. The compiler validates this: **if an agent precedes a branch, it must have an `output` that includes the routing field.**

Compiles to a LangGraph conditional edge:

```python
def route_after_classifier(state):
    return state["category"]

graph.add_conditional_edges("classifier", route_after_classifier, {
    "urgent":  "escalator",
    "normal":  "auto_reply",
    "spam":    "archive",
})
```

### Parallel execution

```hcl
workflow gather {
  [researcher_a, researcher_b, researcher_c] -> aggregator("Synthesize findings")
}
```

Square brackets = parallel. All three run concurrently, results merge into the aggregator.

### Loops / feedback

```hcl
workflow iterative_writing {
  writer("Write the first draft") -> reviewer("Review the draft") -> {
    approved: done
    revise:   writer("Revise based on feedback")
  }
}
```

The `done` keyword is a built-in terminal node (maps to `END`).

### Nested workflows

```hcl
workflow inner_review {
  writer("Write") -> reviewer("Review")
}

workflow outer {
  researcher("Research") -> inner_review -> publisher("Publish")
}
```

A workflow can reference another workflow as a node. This compiles to a LangGraph subgraph.

## Delegation (manager/fan-out pattern)

The manager pattern — where one agent dynamically decides which agents to call — is an **agent-level behavior**, not a graph topology. Use the `delegate` field:

```hcl
agent manager {
  model    = gpt4
  persona  = "Coordinate research across domains"
  delegate = [finance_researcher, tech_researcher, legal_researcher]
  output   = schema {
    synthesis = str
  }
}

workflow research {
  manager("Analyze the market opportunity") -> synthesizer("Write final report")
}
```

The manager agent can invoke any of its delegate agents at runtime. Under the hood, delegate agents are exposed as tools that the manager can call. The manager's internal orchestration (which delegates it calls, how many times) stays encapsulated — only its `output` crosses into the workflow state.

This keeps the workflow graph simple while supporting dynamic fan-out.

## State management

### State is inferred from agent outputs

The compiler walks the workflow graph, collects all agent `output` fields, and generates the `StateGraph` type automatically.

Given:
```hcl
agent researcher {
  output = schema { findings = str, sources = list[str] }
}
agent writer {
  output = schema { report = str }
}
agent reviewer {
  output = schema { approved = bool, feedback = str }
}
```

The compiler generates:
```python
class ReportState(TypedDict):
    messages: Annotated[list, add_messages]  # always present
    findings: str       # from researcher
    sources: list[str]  # from researcher
    report: str         # from writer
    approved: bool      # from reviewer
    feedback: str       # from reviewer
```

Rules:
1. `messages` is always in state (the conversation thread for unstructured agents).
2. Each agent's `output` fields are merged into the workflow state.
3. If two agents define the same field name with different types — **compiler error**.
4. Agents without `output` only contribute to `messages`.
5. Branch routing fields must exist in the preceding agent's `output`.

### Data injection into agent prompts

Each agent receives **what's available in state at that point in the graph**, not everything. The compiler traces the graph to determine which fields are populated by the time each node runs.

For structured agents (with `output`):
- The agent gets a fresh message thread (not shared history).
- State fields from upstream agents are injected into the prompt context.
- The agent's `persona` is the system message.
- The workflow prompt `("...")` is the human message, enriched with available state data.

For unstructured agents (no `output`):
- The agent receives the shared `messages` list.
- State fields are not explicitly injected (the conversation carries the context).

### Manual state override (advanced)

For rare cases where inferred state isn't enough:

```hcl
workflow report {
  state = schema {
    topic    = str
    findings = str
    report   = str
    custom   = int   # not from any agent
  }
  researcher("Research") -> writer("Write")
}
```

When `state` is provided, it replaces inference entirely. The compiler validates that agent outputs are compatible with the declared state.

## Workflow properties

```hcl
workflow report_pipeline {
  researcher("Research") -> writer("Write") -> reviewer("Review")

  timeout = "5m"
}
```

## Pipeline shorthand

A pipeline is NOT a separate concept — it's just a linear workflow. No need for a `pipeline` keyword:

```hcl
# This IS a pipeline. No special syntax needed.
workflow etl {
  extractor("Extract") -> transformer("Transform") -> loader("Load")
}
```

## Compilation strategy

Each workflow compiles to:

1. A Python `TypedDict` for state (inferred from agent outputs or declared manually).
2. Node functions wrapping each agent invocation.
3. A `StateGraph` with edges matching the `->` chains.
4. Conditional edges for `{ label: node }` branch blocks.
5. `START` and `END` sentinel connections.
6. Structured output via Pydantic `BaseModel` for agents with `output`.

### Source mapping

Every generated line includes a comment with the source `.oc` location:

```python
graph.add_node("researcher", researcher_node)  # agents.oc:42:3
graph.add_edge("researcher", "writer")          # agents.oc:45:3
```

This enables stack traces from Python to map back to Orca source.

## Full example

### Input: `agents.oc`

```hcl
model gpt4 {
  provider    = "openai"
  model_name  = "gpt-4o"
  temperature = 0.2
}

tool web_search {
  name = "web_search"
}

agent researcher {
  model   = gpt4
  persona = "You are a research analyst. Find comprehensive information on the given topic."
  tools   = [web_search]
  output  = schema {
    findings = str
    sources  = list[str]
  }
}

agent writer {
  model   = gpt4
  persona = "You are a technical writer. Produce clear, well-structured reports."
  output  = schema {
    report = str
  }
}

workflow report {
  researcher("Research quantum computing advances in 2025")
    -> writer("Write a report from the research findings")
}
```

### Output: `build/report_workflow.py`

```python
# Auto-generated by Orca compiler — do not edit
# Source: agents.oc

from typing import Annotated, TypedDict
from langgraph.graph import StateGraph, START, END
from langgraph.graph.message import add_messages
from langchain_openai import ChatOpenAI
from pydantic import BaseModel


# --- Structured outputs ---

class ResearcherOutput(BaseModel):                        # agents.oc:14:3
    findings: str
    sources: list[str]

class WriterOutput(BaseModel):                            # agents.oc:22:3
    report: str


# --- State ---

class ReportState(TypedDict):                             # agents.oc:26:1
    messages: Annotated[list, add_messages]
    findings: str
    sources: list[str]
    report: str


# --- Models ---

gpt4 = ChatOpenAI(model="gpt-4o", temperature=0.2)       # agents.oc:1:1


# --- Nodes ---

def researcher_node(state: ReportState) -> dict:          # agents.oc:10:1
    """You are a research analyst. Find comprehensive information on the given topic."""
    llm = gpt4.with_structured_output(ResearcherOutput)
    # Inject available state into prompt context
    context_parts = []
    prompt = "Research quantum computing advances in 2025"
    if context_parts:
        prompt = "\n".join(context_parts) + "\n\n" + prompt
    response = llm.invoke([
        {"role": "system", "content": "You are a research analyst. Find comprehensive information on the given topic."},
        {"role": "human", "content": prompt},
    ])
    return {"findings": response.findings, "sources": response.sources}

def writer_node(state: ReportState) -> dict:              # agents.oc:18:1
    """You are a technical writer. Produce clear, well-structured reports."""
    llm = gpt4.with_structured_output(WriterOutput)
    # Inject available state into prompt context
    context_parts = []
    context_parts.append(f"Findings: {state['findings']}")
    context_parts.append(f"Sources: {state['sources']}")
    prompt = "Write a report from the research findings"
    if context_parts:
        prompt = "\n".join(context_parts) + "\n\n" + prompt
    response = llm.invoke([
        {"role": "system", "content": "You are a technical writer. Produce clear, well-structured reports."},
        {"role": "human", "content": prompt},
    ])
    return {"report": response.report}


# --- Graph ---

graph = StateGraph(ReportState)                           # agents.oc:26:1
graph.add_node("researcher", researcher_node)             # agents.oc:27:3
graph.add_node("writer", writer_node)                     # agents.oc:27:3
graph.add_edge(START, "researcher")                       # agents.oc:27:3
graph.add_edge("researcher", "writer")                    # agents.oc:27:3
graph.add_edge("writer", END)                             # agents.oc:27:3

app = graph.compile()
```

## Resolved decisions

1. **State inference**: State is inferred from agent `output` blocks. Manual `state` override available for advanced use.
2. **Message isolation**: Determined by whether an agent has `output`. Structured agents are isolated; conversational agents share messages.
3. **Manager pattern**: Handled by `delegate` field on agents, not graph topology. Delegates are exposed as tools.
4. **Task block**: Not needed. Agent defines the capability (persona, model, tools); workflow provides the task-specific prompt via `("prompt")` syntax.
5. **Inline schemas**: Supported via `schema { ... }` expression syntax. Can be nested.

## Open questions

1. **Human-in-the-loop**: How to express "pause here and wait for human input"? Possible syntax: `-> human ->` as a built-in node.
2. **Checkpointing**: LangGraph supports persistence/checkpointing. Should Orca expose this? E.g., `checkpoint = "sqlite"` on the workflow.
3. **Swarm/decentralized mode**: Agent-driven workflows where agents hand off dynamically. Deferred — may use `mode = swarm` on workflow blocks when ready.
4. **Bounded loops**: `max_iterations` on workflow or per-edge `@max(N)` annotation for safety valves on feedback loops.
5. **Explicit input wiring**: `-> writer("Write", input = [findings])` to restrict which state fields an agent receives. Deferred — auto-injection covers most cases.

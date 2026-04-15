---
layout: doc
sidebar: false
aside: false
blog: post
title: Why Orca?
date: 2026-04-15
category: Personal
description: Frameworks like LangGraph, AutoGen, and CrewAI make you express agent systems in imperative Python. Orca is a small HCL-inspired DSL that lets you declare them instead — and a compiler handles the rest.
---

![Orca](/blog-why-orca-cover.png)

Frameworks like LangGraph, AutoGen, and CrewAI give you the primitives for building AI agent systems, but they force you to express everything in imperative Python. A simple two-agent pipeline easily spans 100+ lines — importing modules, defining state schemas, instantiating models, writing node functions, wiring a graph, and compiling it.

I kept writing the same boilerplate, and every time I did, the actual system — the *interesting* part — disappeared under the scaffolding around it. So I stopped and built a language.

## The idea

Orca is a domain-specific language for AI agent orchestration. You **declare** what your agent system looks like, and a compiler handles the rest. Think [Terraform](https://www.terraform.io/) for AI agents — an HCL-inspired block syntax where you describe *what* exists, not *how* to wire it.

Here's a cron-triggered research-and-write pipeline in Orca:

```orca
model claude {
  provider    = "anthropic"
  model_name  = "claude-opus-4.6"
  temperature = 0.3
}

agent researcher {
  model   = claude
  tools   = [builtins.web_search]
  persona = "You're a tech trends researcher"
}

agent writer {
  model   = claude
  persona = "You're a professional writer"
}

cron daily {
  schedule = "0 9 * * 1-5"
}

workflow search_and_write {
  daily -> researcher -> writer
}
```

And here's the same pipeline hand-written against LangGraph in Python:

```python
from langchain_anthropic import ChatAnthropic
from langchain_community.tools import TavilySearchResults
from langgraph.graph import StateGraph, MessagesState
from langchain_core.messages import SystemMessage

claude = ChatAnthropic(
    model="claude-sonnet-4-20250514",
    temperature=0.3,
)

search_tool = TavilySearchResults()
claude_with_tools = claude.bind_tools([search_tool])

def researcher(state: MessagesState):
    sys = "Research topics thoroughly."
    messages = [SystemMessage(content=sys)] + state["messages"]
    return {"messages": [claude_with_tools.invoke(messages)]}

def writer(state: MessagesState):
    sys = "Write reports from research."
    messages = [SystemMessage(content=sys)] + state["messages"]
    return {"messages": [claude.invoke(messages)]}

graph = StateGraph(MessagesState)
graph.add_node("researcher", researcher)
graph.add_node("writer", writer)
graph.add_edge("__start__", "researcher")
graph.add_edge("researcher", "writer")
graph.add_edge("writer", "__end__")
app = graph.compile()

# Cron trigger? You're on your own.
```

The Python version isn't *wrong*. It's just that the shape of the system you care about — two agents, a cron trigger, an edge between them — is buried in maybe 20% of the lines. The rest is mechanical. And you get the cron trigger yourself, by wiring up an external scheduler, because LangGraph doesn't know what time is.

## Design principles

A few things I kept coming back to while building this:

**Declarative over imperative.** You describe the components and their relationships. The compiler handles the code generation. No state schemas, no graph wiring, no boilerplate.

**Convention over configuration.** Sensible defaults for everything. A `model` block with just a provider works. You only override what you actually need to customize.

**Composability.** Agents, tools, models, and workflows are independent blocks that compose freely. Build complex systems by combining simple, self-contained pieces — the analyzer walks the reference graph across files, so a workflow in one file can wire agents defined in another.

**Highly orthogonal syntax.** The entire language is declarative blocks with parameters as key–value assignments. One construct, predictable everywhere.

**Language-agnostic backend.** Orca is not a wrapper around LangGraph. It's a language with its own compiler, type system, and semantic analyzer. The current backend targets LangGraph, but the architecture is designed for multiple backends — CrewAI and AutoGen are on the list.

## What's working today

The compiler has four stages: lexer → Pratt parser → semantic analyzer → code generator. Right now all four are implemented end-to-end for LangGraph. What that gets you:

- **Block types** for the full agent surface: `model`, `agent`, `tool`, `workflow`, `cron`, `webhook`, `input`, `schema`, `let`
- **Type-safe by default** — every block reference, field type, and schema is checked at compile time, not runtime. Undefined names, type mismatches, and missing required fields all surface before any LLM is invoked.
- **Constant folding** — expressions that reduce to known values are evaluated at compile time and inlined into the generated code. This includes lambda calls with constant arguments, list and map access, and arithmetic. Out-of-range indices and missing keys become compile errors instead of 3 a.m. pages.
- **First-class lambdas with closures** — anonymous functions with type inference, recursion through their enclosing block, and higher-order composition. They compile to Python `lambda` expressions.
- **Custom schemas** — strongly-typed nested user types with field descriptions and annotations, structurally checked against map literals.
- **Built-in triggers** — `cron` and `webhook` blocks wire directly into the generated workflow graph. No external scheduler.
- **Source-mapped diagnostics** — every error points at the exact token; every line of generated Python carries a comment pointing back to the `.orca` file, line, and column that produced it. Debugging the output feels like debugging what you wrote.
- **Language server** — go-to-definition, hover hints, autocomplete, and live diagnostics in any LSP-aware editor. A VS Code extension ships with syntax highlighting.
- **Readable codegen.** The emitted Python is meant to be read, copy-pasted, and extended. No metaprogramming tricks. No opaque runtime.

## What I'm working on next

Three things on the immediate horizon:

1. **Orca Studio** — a browser-based companion to the text language. A Next.js canvas where you lay out models, agents, tools, and workflows visually, and the underlying `.orca` source stays in lockstep. There's a preview in the nav under "Playground".
2. **More backends.** LangGraph is first. CrewAI and AutoGen are next. Because the semantic analyzer runs before codegen, adding a backend is mostly a matter of writing a new visitor — the rest of the pipeline doesn't care.
3. **An agentic compiler.** Natural language in, `.orca` file out, compiled to Python in a second pass. This is the subject of one of two [research papers](https://github.com/ThakeeNathees/orca/tree/main/paper) shipping alongside the project. The idea is that LLM non-determinism is isolated to the *first* stage — turning intent into a valid program — while the second stage, compiling that program to runnable code, is fully deterministic.

If any of this resonates, the [Getting Started guide](/guide/getting-started) is five minutes. If you try it and something breaks, I want to know about it — the repo is on [GitHub](https://github.com/ThakeeNathees/orca).

— Thakee

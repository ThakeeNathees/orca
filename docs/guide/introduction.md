# Introduction

## What is Orca?

Orca is a declarative language for defining AI agents. Instead of writing verbose Python code with frameworks like LangGraph, you describe your agents, models, tools, and workflows in a concise, HCL-like syntax — and Orca compiles it to Python for you.

```orca
model gpt4 {
  provider    = "openai"
  model_name  = "gpt-4o"
  temperature = 0.7
}

agent writer {
  model   = gpt4
  persona = "You are a helpful writer."
}
```

This compiles to fully working Python code with LangGraph, complete with source mapping back to your `.orca` files for easy debugging.

## Why Orca?

Writing AI agent systems in Python means dealing with a lot of boilerplate — setting up models, wiring tools, defining state graphs, and managing orchestration logic. As your system grows, the signal-to-noise ratio drops fast.

Orca lets you focus on **what** your agents do, not **how** to wire them up:

- **Declarative** — Define agents, models, and tools as simple blocks. No class hierarchies or callback functions.
- **Compiled** — Orca compiles to Python/LangGraph code with full source mapping, so you can debug the generated code by tracing back to your `.orca` source.
- **Composable** — Build multi-agent workflows by connecting agents with arrow syntax (`agent1 -> agent2 -> agent3`).
- **Type-safe** — The compiler catches undefined references, type mismatches, and missing fields before you ever run anything.

## How it works

The Orca compiler follows a standard compilation pipeline:

```
.orca files → Lexer → Parser → Analyzer → IR → Code Generation
```

1. **Lexer** — Tokenizes `.orca` source files with line/column tracking.
2. **Parser** — Builds an AST using a Pratt parser with operator precedence.
3. **Analyzer** — Performs semantic analysis: resolves references, checks types, validates schemas.
4. **IR** — Converts the AST to a fully resolved intermediate representation.
5. **Code Generation** — Produces Python code targeting LangGraph.

The generated output includes:
- `build/main.py` — Your agent system as Python code.
- `build/pyproject.toml` — Python project config with the right dependencies.

## Who is Orca for?

Orca is for programmers who want a concise, declarative alternative to writing verbose LangGraph Python code. If you find yourself copy-pasting model setup code, wiring tools to agents, and writing boilerplate orchestration logic — Orca is for you.

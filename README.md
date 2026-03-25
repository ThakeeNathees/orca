<p align="center">
  <img src="docs/logo/logo-bg-white.png" alt="Orca">
</p>

<p align="center">
  <strong>A declarative language for AI agent orchestration.</strong><br>
  Define agents, tools, and workflows in 20 lines — not 200.
</p>

<p align="center">
  <a href="#why-orca">Why Orca</a> &middot;
  <a href="#quick-start">Quick Start</a> &middot;
  <a href="#how-it-works">How It Works</a> &middot;
  <a href="#contributing">Contributing</a>
</p>

---

## Why Orca?

Frameworks like LangGraph, AutoGen, and CrewAI provide the primitives for building AI agent systems, but they require you to express agent configurations, tool bindings, and execution graphs in imperative Python code. A simple two-agent pipeline can easily span 100+ lines — importing modules, defining state schemas, instantiating models, writing node functions, wiring a graph, and compiling it.

Orca is a domain-specific language that lets you **declare** what your agent system looks like, and a compiler handles the rest. Think [Terraform](https://www.terraform.io/) for AI agents — an HCL-inspired block syntax where you describe *what* exists, not *how* to wire it.

<table>
<tr>
<td width="45%" valign="top">

**Orca** (.oc)

```hcl
model claude {
  provider    = "anthropic"
  version     = "claude-sonnet-4-20250514"
  temperature = 0.3
}

tool web_search {
  type = "builtin"
}

agent researcher {
  model  = claude
  tools  = [web_search]
  prompt = "Research topics thoroughly."
}

agent writer {
  model  = claude
  prompt = "Write reports from research."
}

workflow pipeline {
  flow = researcher -> writer
}

trigger daily {
  type     = "cron"
  schedule = "0 9 * * 1-5"
  starts   = pipeline
}
```

</td>
<td width="55%" valign="top">

**LangGraph equivalent** (Python)

```python
from langchain_anthropic import ChatAnthropic
from langchain_community.tools import TavilySearchResults
from langgraph.graph import StateGraph, MessagesState

claude = ChatAnthropic(
    model="claude-sonnet-4-20250514",
    temperature=0.3,
)

search_tool = TavilySearchResults()
claude_with_tools = claude.bind_tools([search_tool])

def researcher(state: MessagesState):
    sys = "Research topics thoroughly."
    messages = [SystemMessage(content=sys)]
        + state["messages"]
    return {"messages":
        [claude_with_tools.invoke(messages)]}

def writer(state: MessagesState):
    sys = "Write reports from research."
    messages = [SystemMessage(content=sys)]
        + state["messages"]
    return {"messages":
        [claude.invoke(messages)]}

graph = StateGraph(MessagesState)
graph.add_node("researcher", researcher)
graph.add_node("writer", writer)
graph.add_edge("__start__", "researcher")
graph.add_edge("researcher", "writer")
graph.add_edge("writer", "__end__")
app = graph.compile()

# Cron trigger? You're on your own.
```

</td>
</tr>
</table>

### Design Principles

**Declarative over imperative.** You describe the components of an agent system and their relationships. The compiler handles the code generation. No state schemas, no graph wiring, no boilerplate.

**Static safety.** The compiler catches misconfigurations — undefined references, type mismatches, missing required fields — at compile time, before any code runs. No more discovering a typo three layers deep in a runtime stack trace.

**Framework independent.** Orca is not a wrapper around LangGraph. It's a language with its own compiler, type system, and semantic analysis. The current backend targets LangGraph, but the architecture is designed for multiple backends (CrewAI, AutoGen, and others). Your `.oc` files stay the same — only the generated output changes.

**Language independent.** The compiler currently generates Python, but the language design is not tied to Python. Future backends can target other languages and runtimes.

**Source-mapped output.** Every line of generated Python is annotated with its originating `.oc` source location. When something goes wrong, you debug in terms of your declarations, not generated code.

## Quick Start

```bash
git clone https://github.com/ThakeeNathees/orca.git
cd orca/compiler
make build
```

Create a file called `main.oc`:

```hcl
model gpt4 {
  provider    = "openai"
  version     = "gpt-4o"
  temperature = 0.7
}

agent assistant {
  model  = gpt4
  prompt = "You are a helpful assistant."
}
```

Compile it:

```bash
./bin/orca build
```

This reads all `.oc` files in the current directory and generates a `build/` directory with runnable Python and LangGraph code.

## How It Works

```
.oc source → Lexer → Parser → Analyzer → Code Generator → Python
```

The Orca compiler is written in Go with a four-stage pipeline.

The **lexer** tokenizes `.oc` source files with full line and column tracking. The **parser** is a [Pratt parser](https://matklad.github.io/2020/04/13/simple-but-powerful-pratt-parsing.html) that produces a typed AST from the token stream, with error-tolerant parsing that can recover and report multiple diagnostics in a single pass. The **analyzer** performs semantic analysis — resolving references between blocks (an agent referencing a model, a workflow referencing agents), type checking assignments against block schemas, and validating required fields. The **code generator** walks the analyzed AST and emits Python targeting the LangGraph framework, with source-map comments on every line tracing back to the original `.oc` source.

The code generator is behind a `Backend` interface. Adding a new target (CrewAI, AutoGen, or a different language entirely) means implementing that interface — the rest of the pipeline stays unchanged.

Every block type in the language — `model`, `agent`, `tool`, `task`, `workflow`, `trigger` — is defined by a **schema** that specifies its fields, types, and constraints. The analyzer validates `.oc` files against these schemas at compile time, catching errors that frameworks would only surface at runtime.

## Contributing

The compiler has 570+ tests across all pipeline stages. TDD is enforced — no code without a failing test first.

```bash
cd compiler
make build          # compile the binary
make test           # run all tests
make lint           # fmt + vet
```

See [CLAUDE.md](CLAUDE.md) for development conventions and detailed project structure.

## References

Orca is accompanied by a research paper: *"Orca: A Declarative Language for AI Agent Orchestration"* (see `paper/`).

## License

[MIT](LICENSE)

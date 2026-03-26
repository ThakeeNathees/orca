<p align="center">
  <img src="docs/logo/logo-bg-white.png" alt="Orca">
</p>

<p align="center">
  <strong>A declarative language for AI agent orchestration.</strong><br>
  A research agent-as-a-code language for expressing agentic systems as declarative programs
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

**Orca**

```hcl
model claude {
  provider    = "anthropic"
  version     = "claude-sonnet-4-20250514"
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

workflow search_and_write {
  flow = researcher -> writer
}

cron daily {
  schedule = "0 9 * * 1-5"
  run      = search_and_write
}
```

</td>
<td width="55%" valign="top">

**Python**

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
  persona = "You are a helpful assistant."
}
```

Compile it:

```bash
orca build
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

## Editor Support

Orca ships with a VS Code extension that provides syntax highlighting, autocomplete, and go-to-definition for `.oc` files.

<img src="docs/public/vscode-extension.png" alt="VS Code extension showing syntax highlighting and autocomplete" width="700">

## Contributing

The compiler has 570+ tests across all pipeline stages. TDD is enforced.

```bash
cd compiler
make build          # compile the binary
make test           # run all tests
make lint           # fmt + vet
```

See [CLAUDE.md](CLAUDE.md) for development conventions and detailed project structure.

## References

Orca is accompanied by a research paper: *"Orca: A Declarative Language for AI Agent Orchestration"* (see [`paper/`](paper/)).

- Harrison Chase et al. (2025) *LangGraph: Multi-Actor Programs with LLMs* [online] Available at https://github.com/langchain-ai/langgraph

- Sirui Hong et al. (2023) *MetaGPT: Meta Programming for A Multi-Agent Collaborative Framework* [online] Available at https://arxiv.org/abs/2308.00352

- Omar Khattab et al. (2023) *DSPy: Compiling Declarative Language Model Calls into Self-Improving Pipelines* [online] Available at https://arxiv.org/abs/2310.03714

- Dawei Gao et al. (2024) *AgentScope: A Flexible yet Robust Multi-Agent Platform* [online] Available at https://arxiv.org/abs/2402.14034

- Mitchell Hashimoto et al. (2014) *HCL: HashiCorp Configuration Language* [online] Available at https://github.com/hashicorp/hcl

- Jieyuan Wu et al. (2023) *AutoGen: Enabling Next-Gen LLM Applications via Multi-Agent Conversation* [online] Available at https://arxiv.org/abs/2308.08155

- João Moura. (2024) *CrewAI: Framework for Orchestrating Role-Playing AI Agents* [online] Available at https://github.com/crewAIInc/crewAI

- Alexey Kladov. (2020) *Simple but Powerful Pratt Parsing* [online] Available at https://matklad.github.io/2020/04/13/simple-but-powerful-pratt-parsing.html

## License

[MIT](LICENSE)

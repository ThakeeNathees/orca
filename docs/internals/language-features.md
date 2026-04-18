# Orca Language Features

A comprehensive reference of all implemented language features in Orca — a declarative language for defining AI agents with HCL-like syntax that transpiles to Python/LangGraph code.

## Program Structure

An `.oc` file is a flat sequence of **named blocks**. There are no imports, no top-level expressions, no general-purpose control flow statements (`if`, `for`, `while`, `match`), and no standalone function definitions. Everything in Orca is a block.

```orca
// Line comments start with //
model gpt4 {
  provider    = "openai"
  model_name  = "gpt-4o"
  temperature = 0.2
}

agent researcher {
  model  = gpt4
  tools  = [web_search]
  persona = "You research topics thoroughly."
}
```

Each block has a **keyword**, a **name**, and a **body** enclosed in braces. The body contains **assignments** (`key = value`) and, in the case of `workflow` blocks, **bare expressions** (agent edge chains).

## Block Types

Orca has 8 implemented block types. Each block type has a schema that defines its valid fields, required vs. optional status, and accepted types. These schemas are self-hosted — defined in Orca syntax in `compiler/types/builtins.oc`.

### `model` — LLM Provider Configuration

Configures a language model for agents to use.

```orca
model gpt4 {
  provider    = "openai"
  model_name  = "gpt-4o"
  temperature = 0.2
}

model claude {
  provider   = "anthropic"
  model_name = "claude-sonnet-4-20250514"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provider` | `str` | yes | LLM provider: `"openai"`, `"anthropic"`, or `"google"` |
| `model_name` | `str \| model` | yes | Model identifier or reference to another model block |
| `api_key` | `str \| null` | no | API key for authentication |
| `base_url` | `str \| null` | no | Custom base URL for the provider |
| `temperature` | `float \| null` | no | Sampling temperature (0.0–1.0) |

### `agent` — Agent Definition

Defines an AI agent with a model, persona, optional tools, and optional structured output.

```orca
agent writer {
  model   = claude
  persona = "You are a technical writer."
  tools   = [web_search, slack]
  output_schema = schema {
    summary = str
    confidence = float
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `model` | `str \| model` | yes | Reference to a model block or string |
| `persona` | `str` | yes | The agent's system prompt / persona |
| `tools` | `list[tool] \| null` | no | List of tool references |
| `output_schema` | `schema \| null` | no | Structured output schema (inline or reference) |

### `tool` — External Tool Integration

Declares a tool that agents can invoke — either a Python import path or inline Python code.

```orca
tool web_search {
  invoke = "langchain_community.tools.web_search.WebSearchTool"
}

tool slack {
  desc   = "Send messages to Slack"
  invoke = "integrations.slack.send_message"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `invoke` | `str` | yes | Dotted Python import path or inline `` ```py `` raw string |
| `desc` | `str \| null` | no | Human-readable description |
| `input_schema` | `schema \| null` | no | Schema for tool input |
| `output_schema` | `schema \| null` | no | Schema for tool output |

Tools with inline Python code use raw strings:

```orca
tool uppercaser {
  invoke = ```py
  def run(text: str) -> str:
      return text.upper()
  ```
}
```

### `workflow` — Agent Orchestration Graph

Defines how agents are chained together using arrow (`->`) expressions.

```orca
workflow content_pipeline {
  researcher -> writer -> reviewer
}
```

Arrow expressions are **bare expressions** inside the workflow body (not assigned to a field). They define sequential edges between agent nodes. Multiple edge chains can appear as separate lines:

```orca
workflow complex_pipeline {
  researcher -> writer
  writer -> reviewer
  reviewer -> publisher
}
```

Multi-line continuation is supported with a leading `->` on the next line:

```orca
workflow long_chain {
  researcher
    -> writer
    -> reviewer
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `str \| null` | no | Display name for the workflow |
| `desc` | `str \| null` | no | Description of the workflow |

Conditional edges and parallel branches are planned but not yet implemented.

### `knowledge` — RAG Data Source

Declares a knowledge source for retrieval-augmented generation.

```orca
knowledge docs {
  desc = "Company knowledge base"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `desc` | `str \| null` | no | Description of the knowledge source |

### `input` — Runtime Input Declaration

Declares a typed input that is provided at runtime.

```orca
input apikey {
  type      = str
  desc      = "The API key for authentication"
  default   = "sk-xxx"
  sensitive = true
}
```

The `type` field accepts primitive types, user-defined schemas, or inline schema blocks:

```orca
input config {
  type = schema {
    region = str
    max_retries = int | null
  }
  desc = "Deployment configuration"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `schema` | yes | The type of this input |
| `desc` | `str \| null` | no | Human-readable description |
| `default` | `any \| null` | no | Default value if none provided |
| `sensitive` | `bool \| null` | no | Whether to mask this value in output |

### `schema` — Custom Type Definition

Defines a named structural type with typed fields.

```orca
schema vpc_data_t {
  region         = str
  instance_count = int
}
```

Fields are typed using type expressions. Optional fields use union with `null`:

```orca
schema analysis_result {
  @desc("The main finding")
  summary    = str
  confidence = float
  details    = str | null
  tags       = list[str]
  metadata   = map[str]
}
```

Schemas can be referenced by name from `agent.output_schema`, `input.type`, `tool.input_schema`, and other schema-typed fields.

### `let` — Named Constants

Declares a block of named constant values that can be referenced elsewhere via dot access.

```orca
let vars {
  name    = "orca"
  count   = 42
  rate    = 3.14
  enabled = false
  nothing = null
  items   = ["a", "b", "c"]
  config  = {"key": "value", "num": 1}
}
```

`let` blocks support all value types: strings, integers, floats, booleans, null, lists, and maps. Values can be referenced with dot access (e.g., `vars.name`), and the compiler performs constant folding through `let` indirection.

## Expressions

Orca has a full expression sublanguage used in field values, workflow edges, and type positions.

### Literals

| Literal | Syntax | Examples |
|---------|--------|----------|
| String | `"..."` | `"hello"`, `"line\nbreak"` |
| Raw string | `` ``` `` or `` ```lang `` | See [Strings](#strings) section |
| Integer | digits | `42`, `0`, `100` |
| Float | digits with `.` | `3.14`, `0.2`, `.5` |
| Boolean | `true` / `false` | |
| Null | `null` | |

### Collections

**Lists** use bracket syntax with comma-separated elements:

```orca
tools = [web_search, calculator, slack]
items = ["a", "b", "c"]
```

**Maps** use brace syntax with colon-separated key-value pairs:

```orca
config = {"key": "value", "num": 1}
metadata = {region: "us-east", tier: "premium"}
```

Map keys can be strings or bare identifiers. Trailing commas are allowed in both lists and maps.

### References and Access

**Identifiers** refer to named blocks:

```orca
agent writer {
  model = gpt4    // reference to a model block named "gpt4"
}
```

**Member access** uses dot notation:

```orca
temperature = vars.default_temp
```

**Subscript access** uses bracket notation:

```orca
first_item = items[0]
value = config["key"]
```

### Binary Operators

| Operator | Meaning | Precedence | Example |
|----------|---------|------------|---------|
| `->` | Workflow edge | Lowest | `researcher -> writer` |
| `\|` | Type union | Low | `str \| null` |
| `+` `-` | Add / subtract | Medium | `count + 1` |
| `*` `/` | Multiply / divide | High | `rate * 100` |
| `.` `[]` `()` | Access / call | Highest | `obj.field`, `list[0]`, `f()` |

### Inline Blocks

Block keywords can appear in expression position to create anonymous inline blocks:

```orca
agent analyst {
  model   = gpt4
  persona = "You analyze data."
  output_schema = schema {
    refined_instruction = str | null
    ambiguities = list[str]
    error = str | null
  }
}
```

All block keywords except `let` can be used as inline blocks.

### Not Supported

The following are **not** part of the expression language:

- String interpolation (`${...}` or `f"..."`)
- Unary minus (`-x`)
- Conditional expressions (`if`/`else`)
- Loops or iteration
- Lambda / anonymous functions

## Type System

### Primitive Types

All primitives are defined as schemas in `compiler/types/builtins.oc`:

| Type | Description |
|------|-------------|
| `str` | String values |
| `int` | Integer values |
| `float` | Floating-point values |
| `bool` | Boolean (`true` / `false`) |
| `null` | The null value |
| `any` | Universal type — compatible with everything |

### Parameterized Types

```orca
list[str]       // list of strings
list[tool]      // list of tool references
map[str]        // map with string values
```

Lists and maps accept a single type parameter in bracket syntax.

### Union Types

The pipe operator `|` creates union types, commonly used for optional fields:

```orca
api_key = str | null        // optional string
model   = str | model       // string or model reference
```

Unions are parsed as binary expressions and flattened during type checking.

### Type Compatibility

The type checker enforces these rules:

- `any` is compatible with every type
- `int` widens to `float` (numeric coercion)
- Union types are checked element-wise
- List/map element types must be compatible
- Schema names must match (empty name = "any schema" for inline blocks)

## Annotations

Annotations decorate blocks and fields with metadata. They appear before the item they annotate.

### Syntax

```orca
@name                      // no arguments
@name("arg1", "arg2")      // with arguments
```

### Built-in Annotations

| Annotation | Target | Purpose |
|------------|--------|---------|
| `@desc("...")` | block, field | Attach documentation / description |
| `@suppress("code")` | block, field | Silence a specific diagnostic code |
| `@sensitive` | block, field | Mark as containing sensitive data |
| `@required` | field | Mark a field as required |

### Annotation on Fields

```orca
agent writer {
  @desc("Chat model ref")
  model   = gpt4
  persona = "You are a helpful writer."
}
```

### Annotation on Blocks

```orca
@suppress("unknown-field")
workflow report_pipeline {
  flow = researcher -> writer -> reviewer
}
```

Annotations compile to `orca.meta()` and `orca.with_meta()` calls in the generated Python output.

## Strings

### Double-Quoted Strings

Standard strings with escape sequences:

```orca
name = "hello world"
multi = "line one\nline two"
```

Supported escapes: `\n` (newline), `\t` (tab), `\\` (backslash), `\"` (quote).

### Raw Strings (Triple Backtick)

Multi-line raw strings use triple backtick delimiters with an optional language tag:

```orca
agent researcher {
  model  = gpt4
  persona = ```md
    You are a research assistant.
    You search the web for information.

    Always cite your sources.
  ```
}
```

Raw strings feature **automatic indentation normalization**: the column of the closing triple backtick determines the baseline indentation, and leading whitespace up to that column is stripped from every line.

The optional language tag (e.g., `md`, `py`) is preserved in the AST for tooling and is used by codegen — for example, `` ```py `` in tool `invoke` fields triggers inline Python function generation.

## Multi-File Compilation

Orca supports splitting a project across multiple `.oc` files in the same directory:

- `orca build` reads all `.oc` files in the current directory
- All files are merged into a single program with a unified symbol table
- Cross-file references resolve automatically — an agent in `agents.oc` can reference a model in `models.oc`
- The LSP server merges sibling `.oc` files for analysis, enabling real-time cross-file diagnostics and completion

## Generated Output

Running `orca build` produces a `build/` directory containing:

| File | Purpose |
|------|---------|
| `build/orca.py` | Runtime support library (embedded in the compiler binary) |
| `build/main.py` | Generated Python code from all `.oc` source files |

### Output Structure

The generated `main.py` follows a fixed section order:

1. Imports (`import orca`, `TypedDict`, LangChain providers, tool imports)
2. Schemas
3. Inputs
4. Variables (`let` blocks)
5. Models
6. Knowledge
7. Tools
8. Graph State (placeholder `TypedDict`)
9. Agents

Each `.oc` block becomes a Python function call:

```python
# model gpt4 { ... }  →
gpt4 = orca.model(
    provider="openai",
    model_name="gpt-4o",
    temperature=0.2,
)

# agent researcher { ... }  →
researcher = orca.agent(
    model=gpt4,
    persona="You are a research assistant.",
    tools=[web_search, calculator],
)
```

The current codegen backend targets **LangGraph** exclusively. Provider strings map to LangChain chat class imports (`ChatOpenAI`, `ChatAnthropic`, `ChatGoogleGenerativeAI`). Future backends for CrewAI and AutoGen are planned.

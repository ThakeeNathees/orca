# Orca Compiler Internals

A detailed walkthrough of the Orca compiler architecture, implementation techniques, and design decisions. Written for contributors and anyone interested in how a domain-specific language compiler works.

## Pipeline Architecture

The compiler transforms `.oc` source files into Python/LangGraph code through four stages:

```
.oc source ──► Lexer ──► Parser ──► Analyzer ──► Codegen ──► build/
               token      ast       types +       langgraph
                                    diagnostic
```

Each stage is a separate Go package under `compiler/`. There is no separate IR (intermediate representation) package -- the analyzed AST serves that role. The pipeline is intentionally linear: each stage consumes the output of the previous one and produces input for the next.

The key data types flowing through the pipeline:

| Stage | Input | Output |
|-------|-------|--------|
| Lexer | raw `.oc` source string | stream of `token.Token` |
| Parser | token stream | `*ast.Program` (AST) |
| Analyzer | `*ast.Program` | `analyzer.AnalyzedProgram` (AST + SymbolTable + Diagnostics) |
| Codegen | `AnalyzedProgram` | `codegen.CodegenOutput` (file tree + dependencies) |

For multi-file compilation, `cmd/build.go` parses each `.oc` file with its own lexer (preserving per-file `SourceFile` paths for diagnostics), then concatenates all statements into a single `ast.Program` before analysis.

## Lexer

**Package:** `compiler/lexer/` -- **Source:** `lexer.go`

The lexer is a classic single-pass character scanner that converts raw source text into a stream of tokens.

### Core State

```go
type Lexer struct {
    input        string
    position     int    // points to current char
    readPosition int    // one ahead of position
    ch           byte   // current character
    line         int    // 1-based line number
    column       int    // 1-based column number
    SourceFile   string // .oc file path for diagnostics
}
```

Every token carries its source position (`Line`, `Column`) plus end position (`EndLine`, `EndCol`) for multi-line tokens. This enables precise diagnostic ranges and LSP features like hover and go-to-definition.

### Techniques

**1-character lookahead:** `peekChar()` returns the next character without advancing, used to distinguish multi-character tokens:
- `-` vs `->` (arrow)
- `/` vs `//` (comment)
- `.` vs `.5` (float starting with decimal point)

**Comment handling:** When `//` is detected, `skipComment` consumes characters until end-of-line, then `NextToken` recurses to return the actual next token.

**Raw string lexing:** Triple backtick strings go through a multi-step process:
1. Detect ```` ``` ```` opening
2. Optionally read a language tag (e.g., `py`, `md`)
3. Consume content until closing ```` ``` ````
4. Run `dedentRawString` -- normalizes indentation by using the closing backtick's column position as the baseline, stripping that many leading spaces/tabs from every line

**Number lexing:** `readNumber` handles both integers and floats. Float detection triggers when a `.` is found after digits (or `.5`-style floats where `.` is followed by a digit). The token type is set to `INT` or `FLOAT` based on whether a decimal point was encountered.

**Token end positions:** `setTokenEnd` computes `EndLine`/`EndCol` from the literal length for single-line tokens. Strings and raw strings set end positions explicitly during scanning since they may span multiple lines.

## Parser

**Package:** `compiler/parser/` -- **Source:** `parser.go`

The parser is a **hybrid recursive descent + Pratt parser**. Recursive descent handles program structure (blocks, assignments, block bodies), while Pratt parsing handles expressions with operator precedence.

### Two-Token Lookahead

```go
type Parser struct {
    l           *lexer.Lexer
    diagnostics []diagnostic.Diagnostic
    prevToken   token.Token // last consumed token (for span ends)
    curToken    token.Token // currently being examined
    peekToken   token.Token // next token (lookahead)
}
```

`prevToken` is critical for tracking AST node spans -- after consuming a closing delimiter like `}` or `]`, the parser needs `prevToken` to set `TokenEnd` on the enclosing node.

### Pratt Expression Parsing

Expressions use Pratt parsing with 5 precedence levels (higher binds tighter):

| Precedence | Operators | Value |
|------------|-----------|-------|
| `PrecLowest` | (none -- stops parsing) | 0 |
| `PrecArrow` | `->` | 1 |
| `PrecPipe` | `\|` | 2 |
| `PrecSum` | `+`, `-` | 3 |
| `PrecProduct` | `*`, `/` | 4 |
| `PrecAccess` | `.`, `[`, `(` | 5 |

The core loop in `parseExpression(precedence)`:
1. Parse a **primary** expression (literal, identifier, list, map, inline block, grouped expression)
2. While the current token's precedence exceeds the caller's precedence, parse the infix operation:
   - Binary operators (`+`, `-`, `*`, `/`, `->`, `|`) produce `BinaryExpression`
   - `.` produces `MemberAccess`
   - `[` produces `Subscription`
   - `(` produces `CallExpression`

All binary operators are **left-associative**. Access operations (`.`, `[`, `(`) bind tighter than any binary operator.

### Block Body Parsing

`parseBlockBody` handles the interior of `{ ... }` blocks, distinguishing between:
- **Assignments**: an identifier-like token followed by `=`, optionally preceded by annotations (`@name`)
- **Bare expressions**: anything else -- primarily workflow edge chains like `A -> B -> C`

The `IsIdentLike` function is key here: block keywords (`model`, `agent`, etc.) and `null` are valid as assignment keys. This allows natural syntax like `model = gpt4` inside an agent block, where `model` is both a keyword and a field name.

### Inline Block Expressions

When a block keyword appears in expression position followed by `{`, the parser creates a `BlockExpression` -- an anonymous inline block:

```orca
output_schema = schema { summary = str }
```

This is handled in `parsePrimary`: if the current token is a block keyword (except `let`) and the peek token is `{`, it branches to `parseBlockExpression`.

### Error Recovery

The parser is designed for IDE resilience -- it produces partial AST even when source code has errors:

- **Skip-one-token safety:** In `ParseProgram`, if `parseStatement` returns nil and the position hasn't advanced, one token is skipped to prevent infinite loops
- **Sync functions:** `syncToBlockEnd` skips to the next `}`, `syncToNextAssignment` skips to the next line with an identifier followed by `=`
- **Partial nodes:** `MemberAccess` allows empty `Member` (for typing `gpt4.` mid-completion), `Subscription` and `CallExpression` allow nil/missing parts
- **`HasErrors` flag:** Set on `Program` so downstream stages know the AST may be incomplete

## AST

**Package:** `compiler/ast/` -- **Source:** `ast.go`

### Node Hierarchy

Every AST node implements the `Node` interface:

```go
type Node interface {
    Start() token.Token
    End() token.Token
}
```

Two marker interfaces separate statements from expressions at the type level:

```go
type Statement interface { Node; statementNode() }
type Expression interface { Node; expressionNode() }
```

`BaseNode` is embedded in all nodes, providing `TokenStart`/`TokenEnd` fields. `NewTerminal(tok)` creates a `BaseNode` where both start and end are the same token -- used for single-token nodes like identifiers and literals.

### Node Types

**Root:**
- `Program` -- holds `[]Statement` (all top-level blocks) and `HasErrors` flag

**Statements:**
- `BlockStatement` -- top-level named block; embeds `BlockBody` plus `Name`, `NameToken`, `OpenBrace`, `Annotations`
- `Assignment` -- `key = value` inside a block body

**Shared structure:**
- `BlockBody` -- holds `Kind`, `Assignments`, `Expressions`, `SourceFile`; shared between `BlockStatement` (top-level) and `BlockExpression` (inline)
- `Annotation` -- `@name` or `@name(args...)`

**Expressions:**
- `Identifier`, `StringLiteral` (with `Lang` field for raw strings), `IntegerLiteral`, `FloatLiteral`, `BooleanLiteral`, `NullLiteral`
- `BinaryExpression` (left/operator/right), `MemberAccess` (object.member), `Subscription` (object[index]), `CallExpression` (callee(args))
- `ListLiteral`, `MapLiteral` (with `MapEntry` key-value pairs)
- `BlockExpression` -- inline anonymous block (embeds `BlockBody`)

### Key Design: BlockBody Sharing

`BlockBody` is the central structural unit -- both `BlockStatement` and `BlockExpression` embed it. This means the analyzer, codegen, and tooling can operate on a single type regardless of whether a block is top-level or inline. The analyzer's `analyzeBlockBody` function validates both through the same code path.

## Semantic Analyzer

**Package:** `compiler/analyzer/` -- **Source:** `analyzer.go`

The analyzer performs three phases of semantic analysis on the parsed AST.

### Phase 1: Symbol Table Construction (`buildSymbolTable`)

1. **Seed builtins:** All built-in schema names (`str`, `int`, `model`, `agent`, etc.) are registered from `types.BuiltinSchemaNames()`. Block-type names resolve to their own kind; primitives resolve to `BlockRef(schema)`.

2. **Register blocks:** Each top-level block's name is added to the symbol table with its `BlockRef` type. Duplicate names produce `duplicate-block` diagnostics (unless suppressed with `@suppress`).

3. **`let` block special handling:** For `let` blocks, the analyzer builds a **per-instance schema** by inferring `ExprType` for each assignment value. This schema is registered with `types.RegisterSchema` so that member access like `vars.name` can resolve field types.

4. **`input` type resolution:** For `input` blocks, `inputDeclaredType` extracts the `type` field's expression type so that the symbol table entry reflects the declared type rather than just "input".

### Phase 2: User Schema Registration (`registerUserSchemas`)

User-defined `schema` blocks are converted to `types.BlockSchema` via `types.SchemaFromBlock` and registered in the global schema registry. This happens after symbol table construction so that schema fields can reference other blocks.

### Phase 3: Per-Block Analysis (`analyzeBlock` / `analyzeBlockBody`)

For each top-level block, the analyzer checks:

- **Duplicate fields:** Two assignments with the same key in one block
- **Unknown fields:** Assignment key not in the block's schema
- **Missing required fields:** Schema-required fields with no assignment (diagnostic range spans from `{` to `}`)
- **Type mismatches:** `types.ExprType` of the value vs. `types.IsCompatible` with the schema field type
- **Reference validation:** Recursive `checkReferences` walk through all expression types
- **Workflow expression validation:** Bare expressions in workflow blocks must be `->` chains of identifiers only
- **Special `invoke` validation:** Tool blocks with inline `` ```py `` invoke strings must contain a `def` matching the block name

### Reference Resolution (`checkReferences`)

A recursive walk that handles every expression type:

- `Identifier` -- looks up in symbol table; reports `undefined-ref` if missing
- `MemberAccess` -- validates object first, then checks the member against the object's schema fields; reports `unknown-member`
- `ListLiteral` -- recurses into each element
- `BinaryExpression` -- recurses into left and right
- `Subscription` -- validates object, then checks index type (must be integer for lists)
- `CallExpression` -- validates callee and each argument
- `MapLiteral` -- validates each key and value
- `BlockExpression` -- recursively calls `analyzeBlockBody` for the inline block (identical validation path as top-level blocks)

### Type Inference (`types/expr_type.go`)

`ExprType` infers the type of any expression node:

- Literals return their primitive type (`str`, `int`, `float`, `bool`, `null`)
- Identifiers look up the symbol table
- Member access resolves the object type's schema, then looks up the field
- Subscripts return the element type of lists or the value type of maps
- `|` binary produces a union type
- Arithmetic operators follow numeric promotion rules
- Inline `BlockExpression` triggers anonymous schema registration

### Type Compatibility (`types.IsCompatible`)

Checks whether an expression type is assignable to an expected type:

- `any` is universally compatible
- `int` widens to `float` (numeric coercion)
- Union types: the expression type must be compatible with at least one union member
- Lists/maps: element types must be compatible
- Schemas: names must match (empty name = wildcard for inline blocks)

### Diagnostic Suppression

`@suppress` annotations on blocks or fields filter diagnostics before they're reported:

- `@suppress` (no args) -- suppresses all diagnostics for that block/field
- `@suppress("code1", "code2")` -- suppresses only the named diagnostic codes

`suppressedCodes` extracts the suppression set; `filterSuppressed` removes matching diagnostics.

## Constant Folding

**Package:** `compiler/analyzer/` -- **Source:** `const_fold.go`

The constant folder evaluates expressions at compile time where possible, producing `ConstValue` results.

### ConstValue

```go
type ConstValue struct {
    Kind     ConstKind              // what type of constant
    Str      string                 // for ConstString
    Int      int64                  // for ConstInt
    Float    float64                // for ConstFloat
    Bool     bool                   // for ConstBool
    List     []ConstValue           // for ConstList
    KeyValue map[string]ConstValue  // for ConstMap and ConstBlock
    Partial  bool                   // true if some sub-values couldn't fold
}
```

The `Partial` flag is notable: it allows representing structures where *some* elements are compile-time constants and others are not. A list like `[1, unknown_ref, 3]` would produce a `ConstList` with `Partial: true`.

### What Folds

- **All literal types:** strings, ints, floats, booleans, null
- **Collections:** lists and maps (recursively folding elements)
- **Block bodies:** `BlockExpression` values fold to `ConstBlock`
- **Arithmetic:** `+`, `-`, `*`, `/` on numeric constants, with int/float promotion (mixed operands promote to float)
- **String concatenation:** `"hello" + " world"` folds to `"hello world"`
- **Identifiers:** Named block references are resolved by finding the block in the AST and re-folding its body
- **Member access:** `block.field` on constant blocks resolves to the field's folded value
- **Subscripts:** `map["key"]` and `list[0]` on constant collections

### What Doesn't Fold

- **`->` and `|` operators:** Workflow edges and type unions are left as `ConstUnknown`
- **Function calls:** `CallExpression` always returns `ConstUnknown` (no pure builtins yet)
- **Division by zero:** Returns `ConstUnknown` silently rather than erroring
- **Member access on null:** Produces an explicit diagnostic

### Usage in Codegen

Constant folding is critical for **provider resolution**: when a model's `provider` field references a `let` variable, the codegen backend uses `ConstFold` to resolve through the indirection:

```orca
let config {
  provider = "openai"
}
model gpt4 {
  provider   = config.provider   // ConstFold resolves this to "openai"
  model_name = "gpt-4o"
}
```

Without constant folding, codegen couldn't determine which LangChain import to generate.

## Code Generation

**Package:** `compiler/codegen/` and `compiler/codegen/langgraph/`

### Backend Architecture

The codegen layer uses an interface pattern for extensibility:

```go
type CodegenBackend interface {
    Generate() CodegenOutput
}

type BaseBackend struct {
    Program      analyzer.AnalyzedProgram
    dependencies []Dependency
}
```

`BaseBackend` provides shared helpers (`CollectBlocksByKind`, `CollectLets`). The only implemented backend is `LangGraphBackend`.

`CodegenOutput` is a tree-structured result:

```go
type CodegenOutput struct {
    BackendType  BackendType        // "langgraph"
    Dependencies []Dependency       // pip packages
    RootDir      OutputDirectory    // build/ directory tree
    Diagnostics  []diagnostic.Diagnostic
}
```

### LangGraph Backend

`LangGraphBackend.Generate()` does three things:
1. `resolveProviders()` -- maps provider strings to LangChain imports via constant folding
2. `resolveToolInvokes()` -- processes tool `invoke` fields (dotted paths or inline Python)
3. Emits `build/orca.py` (embedded runtime) and `build/main.py` (generated code)

### Embedded Runtime

The runtime library `orca.py` is embedded in the Go binary via `//go:embed`:

```go
//go:embed orca.py
var orcaRuntime string
```

It defines Python functions (`orca.model()`, `orca.agent()`, `orca.tool()`, etc.) that create `SimpleNamespace` objects. The generated `main.py` imports this local module. This is a zero-dependency approach -- the runtime is a single file with no external packages.

### Expression Emission (`expr.go`)

`exprToSource` converts AST expressions to Python source code via an exhaustive `switch` on expression types. Key patterns:

- **`blockCallSource`:** Generates `orca.<kind>(key=value, ...)` with optional multi-line formatting
- **`assignmentValueSource`:** Converts a field assignment's value, applying indentation for nested structures
- **`wrapWithMetaIfNeeded`:** When annotations are present, wraps the value in `orca.with_meta(value, [orca.meta(...), ...])`
- **Literal mapping:** `true`/`false` -> `True`/`False`, `null` -> `None`, strings get Python escaping

No template engine is used -- everything is `strings.Builder` and `fmt.Fprintf`.

### Provider Resolution (`providers.go`)

A registry maps provider names to LangChain metadata:

```go
var providerRegistry = map[string]providerInfo{
    "openai":    { /* ChatOpenAI from langchain_openai */ },
    "anthropic": { /* ChatAnthropic from langchain_anthropic */ },
    "google":    { /* ChatGoogleGenerativeAI from langchain_google_genai */ },
}
```

`resolveProviders` walks all model block bodies (including inline ones via `collectBodiesByKind`), applies `ConstFold` to each `provider` field, and collects unique imports. Unknown providers get a `codegen`-stage diagnostic.

### Tool Resolution (`tool.go`)

Two invoke strategies:

1. **Dotted import paths** (e.g., `"langchain_community.tools.web_search.WebSearchTool"`):
   - Split at last `.` into module and callable
   - Generate `from module import callable`
   - Reference the callable by name in `orca.tool(invoke=Callable)`

2. **Inline Python** (`` ```py ... ``` ``):
   - Extract function name via regex
   - Rename the function to `<tool_name>__invoke_verbatim` to avoid collision with the tool variable
   - Emit the function definition verbatim before the `orca.tool(...)` call

### Output Ordering

`generateMain` writes sections in a fixed order regardless of source order:

1. Header comment
2. Imports (`import orca`, `TypedDict`, provider imports, tool imports)
3. Schemas
4. Inputs
5. Variables (`let`)
6. Models
7. Knowledge
8. Tools (with special handling for invoke)
9. GraphState placeholder
10. Agents

### Import Management (`python/python.go`)

`PythonImport` structs represent Python import statements:

```go
type PythonImport struct {
    Module     string         // e.g. "langchain_openai"
    Package    string         // pip package for dependency tracking
    FromImport bool           // true for "from X import Y"
    Symbols    []ImportSymbol // imported names
}
```

Imports are deduplicated and sorted before emission.

## Diagnostics

**Package:** `compiler/diagnostic/` -- **Source:** `diagnostic.go`

### Diagnostic Structure

```go
type Diagnostic struct {
    Severity    Severity  // Error, Warning, Info, Hint
    Code        string    // machine-readable code for suppression
    Position    Position  // start of the range (1-based line/column)
    EndPosition Position  // end of the range (zero = same as Position)
    Message     string    // human-readable description
    Source      string    // pipeline stage: "parser", "analyzer", "codegen"
    File        string    // source .oc file (for multi-file compilation)
}
```

Implements Go's `error` interface with format: `source:line:col: [code] message`.

### Diagnostic Codes

| Code | Stage | Meaning |
|------|-------|---------|
| `syntax` | parser | Parse errors (unexpected token, missing delimiter) |
| `duplicate-block` | analyzer | Two top-level blocks with the same name |
| `duplicate-field` | analyzer | Two assignments with the same key in one block |
| `missing-field` | analyzer | Required field not present in block |
| `unknown-field` | analyzer | Field name not in block's schema |
| `type-mismatch` | analyzer | Value type incompatible with field schema |
| `undefined-ref` | analyzer | Identifier not found in symbol table |
| `unknown-member` | analyzer | Member not found on block type's schema |
| `invalid-subscript` | analyzer | Non-integer index on a list |
| `invalid-value` | analyzer | Field value not in allowed set (e.g., invoke without `def`) |
| `unknown-provider` | codegen | Provider string not in the registry |
| `unsupported-lang` | analyzer | Raw string language tag not supported (only `py` is) |
| `unexpected-expr` | analyzer | Expression not allowed in context (e.g., arithmetic in workflow) |

### LSP Conversion

The LSP server converts diagnostics to LSP protocol format:
- Severity maps to `DiagnosticSeverity` (1=Error, 2=Warning, 3=Info, 4=Hint)
- Positions convert from 1-based (compiler) to 0-based (LSP protocol)
- `Code` becomes an optional diagnostic code for editor display
- `File` is used to route diagnostics to the correct open document

## LSP Server

**Package:** `compiler/lsp/` -- **Source:** `server.go`

### Stack

Built on `github.com/tliron/glsp` with LSP protocol 3.16, communicating over **stdio** (launched by `orca lsp`).

### Capabilities

| Feature | Trigger | Implementation |
|---------|---------|----------------|
| **Diagnostics** | Document open/change | Full re-parse + re-analyze on every change |
| **Completion** | Newline, `.` | Field names in blocks; member fields after dot |
| **Hover** | Cursor position | Block name, identifier type, field schema, annotation info |
| **Go-to-Definition** | Cursor on identifier | Symbol table `DefToken` lookup, cross-file `file://` URIs |

### Document State

```go
type documentState struct {
    Text        string
    Program     *ast.Program
    Symbols     *types.SymbolTable
    Diagnostics []diagnostic.Diagnostic
}
```

Each open file has its own `documentState`. On every change, the server:
1. Parses the updated text (full document sync)
2. Merges sibling `.oc` files from the same directory for a unified symbol table
3. Runs the analyzer even if there are parse errors (partial AST tolerance)
4. Publishes diagnostics filtered to the current file
5. Refreshes sibling file diagnostics (since symbol changes in one file affect others)

### Completion

Uses `cursor.Resolve` to determine positional context:
- **Inside block body:** Offers field names from the block's schema (filtering already-present fields)
- **After `.`:** Uses `cursor.FindNodeAt` with `DotCompletion` to find the object's `ExprType`, then looks up the schema's fields for member completion
- Completion items include `@desc` annotation values as documentation

### Hover

`FindNodeAt` identifies what the cursor is on:
- **Block name:** Shows block kind
- **Identifier:** Shows the symbol's type from the symbol table
- **Member access:** Shows the field's schema type from the object's schema
- **Assignment key:** Shows the field's schema type and `@desc` documentation

### Go-to-Definition

Identifier references resolve to the `DefToken` stored in the symbol table at registration time. For cross-file definitions, the LSP constructs a `file://` URI from the block's `SourceFile` path.

Member access goes through a multi-step resolution:
1. Resolve the object to a block
2. Find the block's assignments
3. For `input` blocks with inline schema types, follow through `type = schema { ... }` indirection

## Cursor Context

**Package:** `compiler/cursor/` -- **Source:** `context.go`

This package centralizes all position-to-semantic-context resolution, serving as the single entry point for LSP features.

### `Resolve(program, line, col) -> Context`

Determines where the cursor sits structurally:

```go
type Context struct {
    Position    CursorPosition     // TopLevel, BlockBody, or FieldValue
    Block       *ast.BlockStatement
    InlineBlock *ast.BlockBody     // non-nil when inside an inline block
    BlockKind   token.BlockKind
    Schema      *types.BlockSchema
    Assignment  *ast.Assignment    // non-nil when cursor is on a value
}
```

The resolution walks top-level blocks, checks if the cursor falls within a block's range, then checks each assignment for finer positioning. For inline `BlockExpression` values, it recursively resolves into the nested block body.

### `FindNodeAt(program, symbols, line, col) -> NodeAt`

Returns a typed discriminator identifying the exact node at the cursor:

- `BlockNameNode` -- cursor on a block's name identifier
- `IdentNode` -- cursor on an identifier reference
- `MemberAccessNode` -- cursor on a member access (with `DotCompletion` flag for cursor immediately after `.`)
- `FieldNameNode` -- cursor on an assignment key

Multi-line token spans are handled via `EndLine`/`EndCol`, ensuring accurate positioning inside raw strings and multi-line values.

## Self-Bootstrapped Type System

**Package:** `compiler/types/` -- **Source:** `builtins.oc` (embedded), `builtins.go`, `schema.go`

One of the most interesting design choices: built-in types are defined in **Orca's own syntax**.

The file `compiler/types/builtins.oc` defines all primitive types and block schemas as `schema` blocks:

```orca
schema str {}
schema int {}
schema float {}
// ... other primitives

schema model {
  provider    = str
  model_name  = str | model
  api_key     = str | null
  temperature = float | null
}

schema agent {
  model         = str | model
  persona       = str
  tools         = list[tool] | null
  output_schema = schema | null
}
```

At Go init time, the `types` package uses `//go:embed builtins.oc` to load and parse this file through the same lexer and parser the compiler uses for user code. The parsed schemas bootstrap the global schema registry that the analyzer then uses for validation.

This self-hosting approach means:
- Block schemas are maintained in Orca syntax, not Go struct definitions
- Adding a new field to a block type only requires editing `builtins.oc`
- The builtins use `@suppress("duplicate-block")` annotations because schema names like `model` collide with the primitive type namespace

## Notable Design Decisions

### `|` is Union, Not Bitwise OR

The pipe operator exclusively creates type unions (`str | null`, `str | model`). There is no bitwise OR for integers. This is an intentional design choice documented in `types/expr_type.go` -- in a language focused on declarative agent definitions, union types are far more useful than bitwise operations.

### Block Keywords as Identifiers

`token.IsIdentLike` returns true for block keywords and `null`, allowing them to appear as assignment keys. This is essential for natural syntax:

```orca
agent writer {
  model = gpt4      // "model" is a keyword AND a valid field name
  tools = [search]  // "tool" could be a field name too
}
```

Without this, the parser would need special cases or a different syntax for field names that happen to be keywords.

### No IR Layer

The pipeline goes directly from analyzed AST to codegen. The `AnalyzedProgram` (AST + SymbolTable + Diagnostics) serves as the IR. The rationale is simplicity -- with a single codegen backend and relatively straightforward translation, a separate IR would add complexity without clear benefit.

If multiple backends diverge significantly in the future, introducing an IR between analysis and codegen would be the natural next step.

### Partial Constants

The `ConstValue.Partial` flag allows the constant folder to represent partially-foldable structures. A list like `[1, some_ref, 3]` produces `ConstList` with `Partial: true` -- the known elements are folded, but the structure is flagged as incomplete. This is more useful than an all-or-nothing approach, since codegen can still extract known values from partial structures (e.g., resolving a provider through a `let` block that has some non-constant fields).

### Error Tolerance Throughout

The entire pipeline is designed for error tolerance:
- The **lexer** emits `ILLEGAL` tokens for unrecognized characters rather than aborting
- The **parser** produces partial AST and continues past errors
- The **analyzer** runs on partial AST (with `HasErrors`), providing what diagnostics it can
- The **LSP** runs the full pipeline on every keystroke, even on broken code

This means the IDE always has some level of analysis available -- completions and hovers work even when the code doesn't parse cleanly.

### Embedded Python Runtime

Rather than requiring users to install a separate `orca` Python package, the runtime is a single `orca.py` file embedded in the Go compiler binary and written to `build/orca.py` during compilation. The runtime uses only Python stdlib (`types.SimpleNamespace`, `typing`), keeping the generated code self-contained.

# Orca

A declarative language for defining AI agents. HCL-like syntax that transpiles to Python code.

File extension: `.oc`

## Repository structure

```
orca/
├── compiler/          # Go — the Orca compiler
├── docs/              # VitePress documentation site
├── editor/            # Editor integrations (VS Code extension)
├── experiments/        # Python experiments and prototypes
├── paper/             # LaTeX research papers
│   ├── agents-as-code/    # "Orca: A Declarative Language for AI Agent Orchestration"
│   └── compiling-intent/  # "Compiling Intent: An Agentic Compiler for Multi-Agent System Generation"
└── CLAUDE.md
```

## Docs (`docs/`)

VitePress documentation site hosted on GitHub Pages.

- **Dev**: `cd docs && pnpm run dev`
- **Build**: `cd docs && pnpm run build`
- **Preview**: `cd docs && pnpm run preview`
- **Auto-maintain**: When a compiler feature is implemented, update the relevant docs page to reflect the new capability. Fill in TODO comments with real content as features are completed.

## Papers (`paper/`)

Two LaTeX research papers:

### `paper/agents-as-code/` — *Orca: A Declarative Language for AI Agent Orchestration*
- **Auto-maintain**: When a compiler feature is implemented, update the relevant section. Remove TODO boxes and fill in content as features are completed.
- **Build**: `cd paper/agents-as-code && make build` — outputs PDF to `out/main.pdf`

### `paper/compiling-intent/` — *Compiling Intent: An Agentic Compiler for Multi-Agent System Generation*
- About the agentic compiler that takes natural language and generates `.oc` files (bootstrapped in Orca itself).
- **Auto-maintain**: When agentic compiler features are implemented in `experiments/orca/`, update the relevant section.
- **Build**: `cd paper/compiling-intent && make build` — outputs PDF to `out/main.pdf`

- **Watch**: `make watch` — auto-rebuilds on changes (in either paper directory)
- Generated files in `*/out/` are gitignored.

## Compiler (`compiler/`)

### Pipeline

```
.oc files → token/lexer → ast/parser → analyzer → ir → codegen (Python)
```

Currently targeting **LangGraph** as the sole codegen backend.

### Packages

- `token/` — token types, precedence levels, and definitions (includes line/column tracking)
- `lexer/` — tokenization of `.oc` source files
- `ast/` — AST node definitions
- `parser/` — Pratt parser producing AST from tokens
- `analyzer/` — semantic analysis (reference resolution, type checking, validation)
- `ir/` — intermediate representation and build logic
- `codegen/` — Python/LangGraph code generation from analyzed AST
- `diagnostic/` — compiler diagnostics (errors, warnings with source locations)
- `cursor/` — cursor context for editor tooling
- `lsp/` — Language Server Protocol implementation
- `cmd/` — CLI commands (`build`, `run`, `lsp`)

### Commands

Run from `compiler/`:

```
go build ./...      # build all packages
go test ./...       # run all tests
go test -v ./...    # verbose test output
make build          # compile binary to bin/orca
make test           # run all tests
make lint           # fmt + vet
```

## Block types

| Block | Purpose |
|-------|---------|
| `model` | LLM provider config (provider, version, temperature, etc.) |
| `agent` | Agent definition (model, tools, prompt) |
| `tool` | External integrations (gmail, slack, notion, web_search, etc.) |
| `task` | Work units assigned to agents |
| `knowledge` | RAG/data sources |
| `workflow` | Agent orchestration graphs |
| `trigger` | Cron, webhook, or event-based execution triggers |

## CLI

- `orca build` — reads all `.oc` files in the current directory, produces a `build/` folder with generated Python code
- `orca run` — builds and runs (future)
- `orca lsp` — starts the language server

## Debug / source mapping

Generated Python code must be fully annotated with source mapping back to the `.oc` file (line, column). This enables debugging generated code by tracing back to the original Orca source.

## Development rules

- **TDD**: Always write tests before implementation. No code without a failing test first.
- **Go conventions**: Follow standard Go style (gofmt, effective Go).
- **Comments**: Always add comments to all exported and unexported functions, types, and non-trivial logic. Comments should explain *why* and *what* for someone reading the code to understand it quickly. Not for end-user docs — for developer comprehension.
- **File layout**: Types and constants at the top of the file, functions at the bottom. Keep declarations before behavior.
- **No Makefiles bloat**: Keep the Makefile minimal and standard.
- **Git**: Commit directly on main. Small, atomic commits. Never mention Claude, add Co-Authored-By, or include any AI attribution in commit messages.
- **Commit messages**: Always start with a relevant Unicode emoji. Pick an icon that reflects the change — e.g. 🐛 bug fix, 🔐 auth/security, 🔧 config/tooling, ✨ new feature, ♻️ refactor, 📝 docs, 🧪 tests, 🎨 UI/style, ⚡ performance, 🗑️ removal, 📦 dependencies, 🚀 deploy/release. **Do not overuse ✨** — use it only for genuinely new features, not for every change. Vary emojis so they're meaningful at a glance.

## Testing rules (enforced)

- **Table-driven tests**: All tests must use Go table-driven test pattern (`[]struct` with `name`, `input`, `expected`).
- **Lexer tests**: Every token type must have a test case. Test input strings against expected `[]token.Token` sequences including `Line` and `Column`.
- **Parser tests**: Every AST node type must have a test case. Use helpers like `parseOrFail(t, input)`, `assertBlockCount()`, `assertBlockType()`.
- **Codegen tests**: Use golden files in `testdata/golden/`. Input `.oc`, expected `.py` (output). Update with `go test -update-golden`.
- **Error cases**: Every stage must test invalid input — bad syntax, undefined references, type mismatches.
- **Integration tests**: End-to-end `.oc` → `build/` output, verify generated Python is valid.
- **No test without assertion**: Every test case must assert something meaningful. No empty or placeholder tests.

## Target audience

Programmers who want a concise, declarative alternative to writing verbose LangGraph Python code. A visual UI (node/edge editor) is planned as a future frontend that generates `.oc` files.

## Important notes

- If the user is wrong, suggest the correct approach rather than just taking orders.

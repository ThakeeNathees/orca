# Orca

A declarative language for defining AI agents. HCL-like syntax that transpiles to Python code via a JSON/YAML intermediate representation (IR).

File extension: `.oc`

## Project status

Early stage — lexer is implemented, parser/AST/evaluator are stubs. Next milestone: parser implementation.

## Architecture

```
.hv files → Lexer → Parser → AST → IR (JSON/YAML) → Backend codegen (Python)
```

The IR is backend-agnostic. Currently targeting **LangGraph** as the sole backend, but the IR layer is designed to support multiple backends in the future.

Packages:
- `token/` — token types and definitions (includes line/column tracking)
- `lexer/` — tokenization of `.oc` source files
- `parser/` — Pratt parser producing AST from tokens
- `ast/` — AST node definitions
- `evaluator/` — IR generation and Python codegen

## Block types

| Block | Purpose |
|-------|---------|
| `model` | LLM provider config (provider, version, temperature, etc.) |
| `agent` | Agent definition (model, tools, prompt) |
| `tool` | External integrations (gmail, slack, notion, web_search, etc.) |
| `task` | Work units assigned to agents |
| `knowledge` | RAG/data sources |
| `workflow` | Agent orchestration graphs (see `docs/`) |
| `trigger` | Cron, webhook, or event-based execution triggers |

## CLI

- `orca build` — reads all `.oc` files in the current directory, produces a `build/` folder with generated Python code
- `orca run` — builds and runs (future)

## Debug / source mapping

Generated Python code must be fully annotated with source mapping back to the `.oc` file (line, column). This enables debugging generated code by tracing back to the original Orca source.

## Development rules

- **TDD**: Always write tests before implementation. No code without a failing test first.
- **Go conventions**: Follow standard Go style (gofmt, effective Go).
- **Comments**: Always add comments to all exported and unexported functions, types, and non-trivial logic. Comments should explain *why* and *what* for someone reading the code to understand it quickly. Not for end-user docs — for developer comprehension.
- **File layout**: Types and constants at the top of the file, functions at the bottom. Keep declarations before behavior.
- **No Makefiles bloat**: Keep the Makefile minimal and standard.
- **Git**: Commit directly on main. Small, atomic commits.
- **Dependencies**: Prefer stdlib when straightforward; use third-party only when it provides significant value.

## Testing rules (enforced)

- **Table-driven tests**: All tests must use Go table-driven test pattern (`[]struct` with `name`, `input`, `expected`).
- **Lexer tests**: Every token type must have a test case. Test input strings against expected `[]token.Token` sequences including `Line` and `Column`.
- **Parser tests**: Every AST node type must have a test case. Use helpers like `parseOrFail(t, input)`, `assertBlockCount()`, `assertBlockType()`.
- **Codegen / evaluator tests**: Use golden files in `testdata/golden/`. Input `.oc`, expected `.json` (IR) and `.py` (output). Update with `go test -update-golden`.
- **Error cases**: Every stage must test invalid input — bad syntax, undefined references, type mismatches.
- **Integration tests**: End-to-end `.oc` → `build/` output, verify generated Python is valid.
- **No test without assertion**: Every test case must assert something meaningful. No empty or placeholder tests.

## Commands

```
go build ./...      # build all packages
go test ./...       # run all tests
go test -v ./...    # verbose test output
make build          # compile binary to bin/orca
make test           # run all tests
make lint           # fmt + vet
```

## Target audience

Programmers who want a concise, declarative alternative to writing verbose LangGraph Python code. A visual UI (node/edge editor) is planned as a future frontend that generates `.oc` files.

## Important notes

- If the user is wrong, suggest the correct approach rather than just taking orders.
- Design proposals and syntax explorations go in `docs/`.

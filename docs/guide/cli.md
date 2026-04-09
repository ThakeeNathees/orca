# CLI Reference

## `orca build`

Compiles all `.oc` files in the current directory and generates Python code.

```bash
orca build
```

**Output:**
- `build/main.py` — Generated Python source code with source mapping comments.
- `build/pyproject.toml` — Python project configuration with dependencies.

**Behavior:**
- Reads every `.oc` file in the current working directory.
- Runs the full pipeline: parse → analyze → generate.
- Reports diagnostics (errors, warnings) to stderr.
- Exits with a non-zero code if there are errors.

**Example output:**
```
compiled 3 .oc file(s) → build/main.py, build/pyproject.toml
```

## `orca lsp`

Starts the Orca Language Server Protocol server for editor integration.

```bash
orca lsp
```

The LSP server communicates over stdin/stdout and provides:
- Diagnostics (errors, warnings) as you type.
- Integration with VS Code and other editors that support LSP.

## `orca run`

Builds and runs the generated Python code.

```bash
orca run
```

**Behavior:**
- Runs the same compile pipeline as `orca build`.
- Runs `uv sync` in the generated `build/` directory.
- Runs `uv run main.py` in the `build/` directory.
- Passes runtime arguments through to `uv run main.py`.

**Examples:**
```bash
orca run p1 p2 p3
orca run -- --foo bar
```

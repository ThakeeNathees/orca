# Compile-Time Analysis

Orca is statically analyzed before codegen. The analyzer walks every block, resolves references, checks types, and reports problems with source locations — so bugs surface as compiler errors rather than Python stack traces at 3 a.m.

## What it catches

### Unknown references

```orca
agent writer {
  model = gpt4    // error: no block named `gpt4`
}
```

The analyzer knows every declared block in every file and refuses to emit Python with a dangling name.

### Type mismatches

```orca
model gpt4 {
  provider    = "openai"
  temperature = "hot"    // error: expected number, got string
}
```

Structural type checking covers primitives, lists, maps, unions, `callable`, and user-defined `schema` blocks.

### Schema compatibility

```orca
schema User {
  name  string
  email string
}

let u {
  current = { name: "Ada" }   // error: missing required field `email`
}
```

Map literals are checked structurally against their expected schema. Missing fields, extra fields, and wrong value types all fail at compile time.

### Invalid block wiring

```orca
agent researcher { model = gpt4 }
model gpt4 { provider = "openai" }

workflow main {
  researcher -> nonexistent    // error: unknown block / invalid node
}
```

Workflow edges, trigger bindings, and tool references are all verified before the graph is emitted.

**String node ids.** In a `workflow` block you may write a string literal as a node (for example `A -> "alias" -> B`) only when `"alias"` is registered as a key in that workflow’s `nodes` map. The analyzer rejects unknown string ids the same way it rejects non–workflow-node blocks.

### Unsupported expressions in the wrong place

Some expressions are only legal inside `workflow` blocks (for example, node references in edges). Use one outside its allowed context and you get a precise error at the exact token.

## Source-mapped diagnostics

Every diagnostic carries a file, line, and column pointing at the offending token — not the enclosing statement, not the enclosing block. The same source location travels through to the generated Python as a `# orca: file:line:col` comment, so debugging jumps straight back to your `.orca` source.

## Suppression

For the rare case where you know better than the analyzer, `@suppress` silences a specific diagnostic on a block or field:

```orca
@suppress("unknown-field")
agent researcher {
  experimental_field = true
}
```

Suppression is opt-in per-rule and never hides errors from other rules.

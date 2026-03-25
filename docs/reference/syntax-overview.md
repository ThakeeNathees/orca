# Syntax Overview

Orca uses an HCL-like block syntax. Files use the `.oc` extension.

## Blocks

Everything in Orca is defined as a named block:

```orca
block_type name {
  field = value
}
```

Available block types: [`model`](/reference/model), [`agent`](/reference/agent), [`tool`](/reference/tool), [`task`](/reference/task), [`knowledge`](/reference/knowledge), [`workflow`](/reference/workflow), [`trigger`](/reference/trigger), [`input`](/reference/input), [`schema`](/reference/schema), and [`let`](/reference/let).

## Comments

```orca
// This is a comment
model gpt4 {
  provider   = "openai"    // inline comment
  model_name = "gpt-4o"
}
```

## Types

### Primitives

| Type | Description |
|------|-------------|
| `str` | String |
| `int` | Integer |
| `float` | Floating-point number |
| `bool` | Boolean (`true` / `false`) |
| `null` | Null value |
| `any` | Matches any type |

### Collections

```orca
// List
tools = [search, calculator, browser]

// Map
headers = {
  content_type: "application/json",
  authorization: "Bearer token"
}
```

- **Lists**: `list[T]` — ordered collection of type `T` (e.g., `list[tool]`).
- **Maps**: `map[T]` — key-value pairs where keys are strings and values are type `T`.

### Unions

Use the pipe operator `|` to allow multiple types:

```orca
model_name = str | model    // accepts a string or a model reference
temperature = float | null  // optional field (null means it can be omitted)
```

A field with `| null` in its type is optional.

### References

Unquoted identifiers reference other blocks:

```orca
model gpt4 {
  provider   = "openai"
  model_name = "gpt-4o"
}

agent writer {
  model = gpt4  // references the model block above
}
```

### Member access

Use dot notation to access fields on referenced blocks:

```orca
agent writer {
  model = gpt4
  persona = "You write for " + config.audience
}
```

### Subscript access

Use brackets for list or map indexing:

```orca
first_tool = my_tools[0]
```

## Expressions

### Literals

```orca
name     = "hello"       // string
count    = 42            // integer
rate     = 0.7           // float
enabled  = true          // boolean
nothing  = null          // null
```

### Arithmetic

```orca
let {
  total = base_price * quantity
  with_tax = total + tax
}
```

Supported operators: `+`, `-`, `*`, `/`.

### Function calls

```orca
result = transform(input, "format")
```

## Annotations

Annotations use the `@` prefix and can decorate blocks or fields:

```orca
@suppress("unknown-field")
agent researcher {
  @desc("The AI model to use")
  model = gpt4
}
```

### `@suppress`

Suppresses compiler diagnostics:

```orca
@suppress                        // suppress all diagnostics
@suppress("type-mismatch")       // suppress specific code
@suppress("unknown-field", "missing-field")  // suppress multiple codes
```

### `@desc`

Adds a description to a field (used in schema definitions):

```orca
schema report {
  @desc("The report title")
  title = str

  @desc("Word count limit")
  max_words = int | null
}
```

### Diagnostic codes

| Code | Description |
|------|-------------|
| `syntax` | Parse errors |
| `duplicate-block` | Two blocks with the same name |
| `duplicate-field` | Repeated field name in a block |
| `missing-field` | Required field not provided |
| `unknown-field` | Field not defined in the block's schema |
| `type-mismatch` | Value type doesn't match field's expected type |
| `undefined-ref` | Identifier not found |
| `unknown-member` | Member not found on referenced block |
| `invalid-subscript` | Non-integer subscript on a list |
| `invalid-value` | Field value not in allowed set |

## Strings

All strings are double-quoted and support escape sequences and multiple lines.

### Escape sequences

| Sequence | Result |
|----------|--------|
| `\n` | Newline |
| `\t` | Tab |
| `\\` | Literal `\` |
| `\"` | Literal `"` |

### Multi-line strings

Any string can span lines. Indentation is automatically stripped based on the closing quote's position:

```orca
agent researcher {
  persona = "
    You are a research assistant.
    You search the web for information.

    Always cite your sources.
    "
}
```

The value of `persona` is:

```
You are a research assistant.
You search the web for information.

Always cite your sources.
```

#### How indentation stripping works

The **closing quote's column** defines the baseline. That many leading spaces are stripped from every content line.

**Rule:** `baseline = closing_quote_column - 1` (columns are 1-based).

```
  persona = "
    Hello               ← 4 spaces before content
      Indented          ← 6 spaces (2 extra will remain)
    World               ← 4 spaces before content
    "                   ← closing " at column 5 → baseline = 4
```

Result:

```
Hello
  Indented
World
```

- Lines indented more than the baseline keep their extra indentation.
- Empty lines in the middle are preserved.
- If the first line after the opening `"` is empty, that empty line is removed.
- The last line (whitespace before the closing `"`) is removed.

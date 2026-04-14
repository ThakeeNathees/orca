# Syntax Overview

Orca uses an HCL-like block syntax. Files use the `.orca` extension.

## Blocks

Everything in Orca is defined as a named block:

```orca
block_type name {
  field = value
}
```

Available block types: [`model`](/reference/model), [`agent`](/reference/agent), [`tool`](/reference/tool), [`workflow`](/reference/workflow), [`cron`](/reference/cron), [`webhook`](/reference/webhook), [`input`](/reference/input), [`schema`](/reference/schema), and [`let`](/reference/let).

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
| `string` | String |
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
- **Maps**: `map[K, V]` — key-value pairs with key type `K` and value type `V` (e.g., `map[string, number]`).
- **Callable**: `callable[P1, P2, ..., R]` — function type with parameter types and return type as the last element (e.g., `callable[number, number, string]` is a function taking two numbers and returning a string).

### Unions

Use the pipe operator `|` to allow multiple types:

```orca
model_name = stringing | model    // accepts a string or a model reference
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
let vars {
  total = base_price * quantity
  with_tax = vars.total + tax
}
```

Supported operators: `+`, `-`, `*`, `/`.

### Comparison operators

```orca
let vars {
  is_valid = count > 0
  is_equal = name == "admin"
  in_range = score >= 50
}
```

Supported operators: `==`, `!=`, `<`, `>`, `<=`, `>=`. Comparison operators have lower precedence than arithmetic but higher than ternary, so `a + 1 < b + 2` parses as `(a + 1) < (b + 2)`.

### Grouped expressions

Use parentheses to override precedence:

```orca
let vars {
  result = (a + b) * c
}
```

### Ternary conditional expressions

Use `condition ? thenExpr : elseExpr` for a conditional value. The condition is any expression; the then and else branches must both be present (a trailing `:` without a value is invalid).

```orca
let vars {
  label = use_short ? "hi" : "hello"
}
```

**Precedence and associativity**

- **Right-associative:** nested ternaries group to the right — `a ? b ? c : d : e` means `a ? (b ? c : d) : e`.
- **Looser than workflow arrows:** `a -> b ? c : d` is parsed as `(a -> b) ? c : d`, not `a -> (b ? c : d)`. When mixing arrows and ternaries, use parentheses if you need the other grouping.

**Types**

If both branches have the same type, that is the expression’s type. If they differ, the compiler forms a flattened union (for example `string | number`).

**Code generation**

Ternary expressions compile to Python’s conditional expression form (`thenExpr if condition else falseExpr`).

### Lambda expressions

Lambda expressions define anonymous functions using `\` visual shorthand for λ (lambda):

```orca
let funcs {
  // With return type annotation
  add = \(a number, b number) number -> a + b

  // Return type inferred
  double = \(x number) -> x * 2

  // Zero parameters
  greet = \() -> "hello"

  // Higher-order (currying)
  add_k = \(k number) -> \(n number) -> k + n
}
```

**Syntax:** `\(param type, ...) return_type -> body`

- The return type is optional — omit it and the compiler infers from the body.
- The body is always a single expression (can span multiple lines).
- `\` is a visual shorthand for λ (lambda)

**Closures:** lambdas capture variables from enclosing scopes:

```orca
let funcs {
  add_k = \(k number) -> \(n number) -> k + n
  add_42 = funcs.add_k(42)
}
```

**Recursion:** lambdas can call themselves through their enclosing block:

```orca
let main {
  // "\" is visual shorthand for λ (lambda)
  fib = \(n number) -> (n > 1)
    ? main.fib(n-1) + main.fib(n-2)
    : n
}
```

**Type:** lambda expressions have type `callable[param_types..., return_type]`. For example, `\(a number, b number) number -> a + b` has type `callable[number, number, number]`.

**Code generation:** lambdas compile to Python `lambda` expressions.

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
  title = string

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
| `invalid-workflow-node` | Block kind not allowed as a workflow node |

## Strings

Orca has two string types: **double-quoted strings** for single-line values and **raw strings** (triple-backtick) for multi-line content.

### Double-quoted strings

Single-line strings with escape sequence support:

```orca
provider = "openai"
greeting = "hello\nworld"
```

| Sequence | Result |
|----------|--------|
| `\n` | Newline |
| `\t` | Tab |
| `\\` | Literal `\` |
| `\"` | Literal `"` |

### Raw strings (triple-backtick)

Multi-line strings use triple backticks with an optional language tag:

~~~orca
agent researcher {
  persona = ```md
    You are a research assistant.
    You search the web for information.

    Always cite your sources.
    ```
}
~~~

The value of `persona` is:

```
You are a research assistant.
You search the web for information.

Always cite your sources.
```

The language tag (`md`, `py`, `json`, `yaml`, etc.) is optional and enables syntax highlighting in editors.

#### How indentation stripping works

The **closing `` ``` ``'s column** defines the baseline. That many leading spaces are stripped from every content line.

**Rule:** `baseline = closing_backtick_column - 1` (columns are 1-based).

```
  persona = ```md
    Hello               ← 4 spaces before content
      Indented          ← 6 spaces (2 extra will remain)
    World               ← 4 spaces before content
    ```                 ← closing ``` at column 5 → baseline = 4
```

Result:

```
Hello
  Indented
World
```

- Lines indented more than the baseline keep their extra indentation.
- Empty lines in the middle are preserved.
- The last line (whitespace before the closing `` ``` ``) is removed.

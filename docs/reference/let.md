# let

The `let` block defines variables and constants that can be referenced throughout your Orca files.

## Syntax

```orca
let <name> {
  key1 = <value>
  key2 = <value>
}
```

Like all other blocks, `let` requires a name. Multiple named let blocks are allowed.

## Examples

### Basic variables

```orca
let vars {
  api_url     = "https://api.example.com"
  max_retries = 3
  temperature = 0.7
  debug       = true
}
```

### Multiple let blocks

```orca
let vars {
  api_key     = "sk-123"
  max_retries = 3
  timeout     = 30
}

let vars2 {
  backup_key = "sk-456"
}
```

### Using variables

Variables defined in `let` are accessed via `block_name.field_name`:

```orca
let vars {
  default_temp = 0.7
}

model gpt4 {
  provider    = "openai"
  model_name  = "gpt-4o"
  temperature = vars.default_temp
}
```

## Generated output

```orca
let vars {
  api_url     = "https://api.example.com"
  max_retries = 3
  temperature = 0.7
  debug       = true
}
```

Compiles to:

```python
vars = orca.let(
    api_url="https://api.example.com",
    max_retries=3,
    temperature=0.7,
    debug=True,
)
```

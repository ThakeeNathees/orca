# let

The `let` block defines variables and constants that can be referenced throughout your Orca files.

## Syntax

```orca
let {
  name1 = <value>
  name2 = <value>
}
```

Unlike other blocks, `let` has no name — it's a single block that holds multiple variable definitions.

## Examples

### Basic variables

```orca
let {
  api_url     = "https://api.example.com"
  max_retries = 3
  temperature = 0.7
  debug       = true
}
```

### Using variables

Variables defined in `let` can be referenced by name anywhere:

```orca
let {
  default_temp = 0.7
}

model gpt4 {
  provider    = "openai"
  model_name  = "gpt-4o"
  temperature = default_temp
}
```

## Generated output

```orca
let {
  api_url     = "https://api.example.com"
  max_retries = 3
  temperature = 0.7
  debug       = true
}
```

Compiles to:

```python
api_url = "https://api.example.com"  # main.oc:2
max_retries = 3  # main.oc:3
temperature = 0.7  # main.oc:4
debug = True  # main.oc:5
```

# schema

The `schema` block defines a custom type with named fields. Schemas are used for structured agent outputs, input types, and type-safe data passing between agents.

## Syntax

```orca
schema <name> {
  field1 = <type>
  field2 = <type>
}
```

## Field types

Each field is assigned a type. Use `| nulltype` to make a field optional — `nulltype` is the type whose only value is `null`:

```orca
schema report {
  title      = string                // required
  summary    = string                // required
  sources    = list[string]          // required
  word_count = number | nulltype     // optional
}
```

## Field descriptions

Use the `@desc` annotation to document fields:

```orca
schema report {
  @desc("The report title")
  title = string

  @desc("Executive summary, max 200 words")
  summary = string

  @desc("List of source URLs")
  sources = list[string]
}
```

## Examples

### Structured output for an agent

```orca
schema analysis {
  sentiment  = string
  confidence = number
  keywords   = list[string]
}

agent analyst {
  model   = gpt4
  persona = "You analyze text sentiment."
  output  = analysis
}
```

### Nested schemas

```orca
schema address {
  street = string
  city   = string
  zip    = string
}

schema customer {
  name    = string
  email   = string
  address = address
}
```

### Schema with collections

```orca
schema search_results {
  query   = string
  results = list[string]
  metadata = map[string]
}
```

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

Each field is assigned a type. Use `| null` to make a field optional:

```orca
schema report {
  title    = str           // required
  summary  = str           // required
  sources  = list[str]     // required
  word_count = int | null  // optional
}
```

## Field descriptions

Use the `@desc` annotation to document fields:

```orca
schema report {
  @desc("The report title")
  title = str

  @desc("Executive summary, max 200 words")
  summary = str

  @desc("List of source URLs")
  sources = list[str]
}
```

## Examples

### Structured output for an agent

```orca
schema analysis {
  sentiment  = str
  confidence = float
  keywords   = list[str]
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
  street = str
  city   = str
  zip    = str
}

schema customer {
  name    = str
  email   = str
  address = address
}
```

### Schema with collections

```orca
schema search_results {
  query   = str
  results = list[str]
  metadata = map[str]
}
```

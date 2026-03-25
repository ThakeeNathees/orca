# knowledge

The `knowledge` block defines a data source for RAG (Retrieval-Augmented Generation).

## Syntax

```orca
knowledge <name> {
  name = <string>  // optional
  desc = <string>  // optional
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `str \| null` | No | Display name for the knowledge source |
| `desc` | `str \| null` | No | Description of the data source |

## Examples

```orca
knowledge company_docs {
  name = "Company Documentation"
  desc = "Internal company policies and procedures"
}

knowledge product_data {
  name = "Product Database"
  desc = "Product catalog with specs and pricing"
}
```

::: info
The knowledge block schema is being expanded. Additional fields for source type, connection details, and indexing configuration are planned.
:::

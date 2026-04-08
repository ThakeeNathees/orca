# knowledge

The `knowledge` block defines a named knowledge source that agents can draw on for retrieval-augmented generation (RAG).

## Syntax

```orca
knowledge <name> {
  name = <string>
  desc = <string>  // optional
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | Yes | The identifier for this knowledge source |
| `desc` | `string \| null` | No | A description of what the knowledge source contains |

## Examples

```orca
knowledge company_docs {
  name = "company_docs"
  desc = "Internal company documentation and policies"
}

knowledge product_faq {
  name = "product_faq"
  desc = "Frequently asked questions about the product"
}
```

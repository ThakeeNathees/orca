# webhook

The `webhook` block defines an HTTP path (and optional method) for a webhook entry point. Use it as a **workflow node** by referencing the block’s name in a [`workflow`](/reference/workflow) edge chain.

## Syntax

```orca
webhook <name> {
  path = <string>
  method = <string>  // optional
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | `string` | Yes | URL path (e.g. `"/hooks/job"`) |
| `method` | `string \| null` | No | HTTP method (e.g. `"POST"`). Omitted in generated code when unset |

## Workflow usage

```orca
webhook hooks_in {
  path   = "/hooks/in"
  method = "POST"
}

agent worker {
  model   = gpt4
  persona = "Process webhook payloads."
}

workflow run {
  hooks_in -> worker
}
```

Use the **block instance name** (`hooks_in`) in edges, not the keyword `webhook`.

## Compilation

The compiler emits an `orca.webhook(...)` value in generated Python and includes the block as a LangGraph node when it appears in a workflow. Serving HTTP and routing to the graph is runtime work beyond `orca build`.

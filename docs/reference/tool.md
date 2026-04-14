# tool

The `tool` block defines an external tool or integration that agents can use.

## Syntax

```orca
tool <name> {
  name         = <string>
  desc         = <string>  // optional
  input_schema = <schema>  // optional
  invoke       = <string>  // optional
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | Yes | The tool's identifier |
| `desc` | `string \| nulltype` | No | A description of what the tool does |
| `input_schema` | `schema \| nulltype` | No | Schema describing the tool's input parameters |
| `invoke` | `string \| nulltype` | No | Fully-qualified Python function to call when the tool is invoked |

## Examples

### Simple tool

```orca
tool search {
  name = "web_search"
  desc = "Search the web for current information"
}

tool gmail {
  name = "gmail"
  desc = "Send and read emails via Gmail"
}

tool slack {
  name = "slack"
  desc = "Send messages to Slack channels"
}
```

### Tool with input schema and invoke

```orca
schema search_input {
  query   = string
  max_results = number | nulltype
}

tool search {
  name         = "web_search"
  desc         = "Search the web for current information"
  input_schema = search_input
  invoke       = "myapp.tools.web_search"
}
```

## Using tools with agents

Reference tool blocks in an agent's `tools` list:

```orca
agent assistant {
  model   = gpt4
  persona = "You help users with tasks."
  tools   = [search, gmail, slack]
}
```

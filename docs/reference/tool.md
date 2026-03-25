# tool

The `tool` block defines an external tool or integration that agents can use.

## Syntax

```orca
tool <name> {
  name = <string>
  desc = <string>  // optional
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `str` | Yes | The tool's identifier |
| `desc` | `str \| null` | No | A description of what the tool does |

## Examples

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

## Using tools with agents

Reference tool blocks in an agent's `tools` list:

```orca
agent assistant {
  model   = gpt4
  persona = "You help users with tasks."
  tools   = [search, gmail, slack]
}
```

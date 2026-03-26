# task

The `task` block defines a unit of work assigned to an agent.

## Syntax

```orca
task <name> {
  agent  = <agent_ref>
  prompt = <string>
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent` | `agent` | Yes | Reference to the agent that performs this task |
| `prompt` | `str` | Yes | The prompt or instruction for the task |

## Examples

```orca
task write_report {
  agent  = writer
  prompt = "Write a summary report on recent AI developments."
}

task review_code {
  agent  = reviewer
  prompt = ```
    Review the following code for:
    - Security vulnerabilities
    - Performance issues
    - Code style violations
    ```
}
```

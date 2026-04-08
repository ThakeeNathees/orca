# cron

The `cron` block defines a cron schedule. Use it as a **workflow node** by referencing the block’s name in a [`workflow`](/reference/workflow) edge chain (alongside [`agent`](/reference/agent), [`tool`](/reference/tool), and [`webhook`](/reference/webhook)).

## Syntax

```orca
cron <name> {
  schedule = <string>
  timezone = <string>  // optional
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `schedule` | `string` | Yes | Cron expression (e.g. `"0 9 * * 1-5"`) |
| `timezone` | `string \| null` | No | IANA timezone (e.g. `"America/New_York"`). Defaults to UTC when unset |

## Workflow usage

Declare a named `cron` block, then use **that name** as the first (or any) node in a workflow:

```orca
cron daily {
  schedule = "0 9 * * 1-5"
  timezone  = "America/New_York"
}

agent researcher {
  model   = gpt4
  persona = "You research topics thoroughly."
}

workflow run {
  daily -> researcher
}
```

The keyword `cron` only starts a block declaration; inside `workflow { ... }`, node references are **identifiers** (the block name `daily`, not the word `cron`).

## Cron expression format

```
┌─ minute        (0–59)
│ ┌─ hour         (0–23)
│ │ ┌─ day of month (1–31)
│ │ │ ┌─ month       (1–12)
│ │ │ │ ┌─ day of week  (0–7, 0 and 7 = Sunday)
│ │ │ │ │
* * * * *
```

| Expression | Description |
|------------|-------------|
| `0 9 * * 1-5` | Every weekday at 9:00 AM |
| `0 * * * *` | Every hour on the hour |
| `*/15 * * * *` | Every 15 minutes |
| `0 0 1 * *` | First day of every month at midnight |

## Compilation

The compiler emits an `orca.cron(...)` value in generated Python and includes the block as a LangGraph node when it appears in a workflow.

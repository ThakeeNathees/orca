# trigger

The `trigger` block defines when and how workflows or tasks are executed.

## Syntax

```orca
trigger <name> {
  name = <string>  // optional
  desc = <string>  // optional
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `str \| null` | No | Display name for the trigger |
| `desc` | `str \| null` | No | Description of the trigger |

## Examples

```orca
trigger daily_report {
  name = "Daily Report"
  desc = "Generates a daily summary report every morning"
}

trigger on_new_ticket {
  name = "New Ticket Handler"
  desc = "Triggered when a new support ticket is created"
}
```

::: info
The trigger block schema is being expanded. Support for cron schedules, webhook endpoints, and event-based triggers is planned.
:::

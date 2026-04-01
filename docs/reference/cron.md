# cron

The `cron` block defines a scheduled trigger for workflows using cron expressions.

## Syntax

```orca
cron <name> {
  schedule = <string>
  workflow = <workflow_ref>
  timezone = <string>  // optional
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `schedule` | `str` | Yes | Cron expression defining the schedule (e.g. `"0 9 * * 1-5"`) |
| `workflow` | `workflow` | Yes | Reference to the workflow to execute |
| `timezone` | `str \| null` | No | IANA timezone name (e.g. `"America/New_York"`). Defaults to UTC |

## Examples

```orca
cron daily_report {
  schedule = "0 9 * * 1-5"
  workflow  = report_pipeline
  timezone  = "America/New_York"
}

cron hourly_sync {
  schedule = "0 * * * *"
  workflow  = data_sync
}
```

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

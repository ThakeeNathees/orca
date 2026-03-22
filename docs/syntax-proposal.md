# Orca Syntax Proposal

## Design goals

1. **Minimal boilerplate** — define an agent system in ~30 lines, not 300
2. **Readable by non-authors** — someone unfamiliar should understand what the system does
3. **Composable** — blocks reference each other by name, not by wiring code
4. **Source-mappable** — every construct maps cleanly to a line/column for debugging

## Block syntax

All top-level constructs follow the same pattern:

```
keyword name {
  key = value
}
```

### Option A: Flat HCL-style (current direction)

```hcl
model gpt4 {
  provider    = "openai"
  version     = "gpt-4o"
  temperature = 0.2
}

tool web_search {
  type = "builtin"  # builtin | api | function
}

tool gmail {
  type    = "oauth"
  scopes  = ["send", "read"]
}

agent researcher {
  model  = gpt4
  tools  = [web_search, gmail]
  prompt = "You are a research assistant."
}

task summarize {
  description     = "Summarize findings into a report."
  assignee        = researcher
  expected_output = "markdown"
}
```

**Pros**: Simple, flat, easy to parse. Familiar to Terraform/HCL users.
**Cons**: No way to express flow logic inline. Workflows need a separate block.

### Option B: Flat + arrow syntax for workflows

Same as Option A for resource blocks, but adds arrow syntax for workflows:

```hcl
workflow research_pipeline {
  researcher -> writer -> reviewer

  on_error = retry(3)
}
```

The `->` operator defines execution order. This is the simplest way to express a DAG.

For branching/conditional flows:

```hcl
workflow support_ticket {
  classifier -> {
    urgent:  escalator -> human_agent
    normal:  auto_responder
    spam:    archive
  }

  on_error = notify(ops_team)
}
```

**Pros**: Very readable. Flow is visual. No need to learn graph APIs.
**Cons**: Slightly more complex parser. Limited expressiveness for very complex graphs.

### Option C: Pipeline shorthand

For simple linear flows, allow a `pipeline` shorthand:

```hcl
pipeline daily_digest {
  steps = [collector, summarizer, formatter, emailer]
}
```

This is syntactic sugar for a workflow where each step feeds into the next.

**Pros**: Ultra-concise for common linear patterns.
**Cons**: Another keyword to learn. Could just be a workflow with `->`.

### Recommendation

**Use Option A + B together.** Keep resource blocks flat and simple. Use arrow syntax for workflows. Skip `pipeline` as a separate keyword — a workflow with `->` is just as clear and avoids redundancy.

---

## Value types

| Type | Example | Notes |
|------|---------|-------|
| String | `"hello"` | Double-quoted only |
| Int | `42` | |
| Float | `0.7` | |
| Bool | `true`, `false` | |
| List | `[a, b, c]` | Can contain references or literals |
| Reference | `gpt4`, `researcher` | Unquoted identifier, resolved to a defined block |
| Dotted ref | `workflow.daily` | For namespaced references (triggers) |

## Multiline strings

For prompts (which are often long), support heredoc-style strings:

```hcl
agent writer {
  model = gpt4
  prompt = <<EOF
    You are a technical writer.
    Write clear, concise documentation.
    Use markdown formatting.
  EOF
}
```

## Comments

```hcl
# This is a comment
model gpt4 {  # inline comment
  provider = "openai"
}
```

Single-line only (`#`). No block comments — keeps the lexer simple.

## Tool definitions

Tools represent external capabilities agents can use:

```hcl
# Built-in tools (provided by the runtime)
tool web_search {
  type = "builtin"
}

# API-based tools
tool slack {
  type     = "api"
  base_url = "https://slack.com/api"
  auth     = "oauth"
  scopes   = ["chat:write", "channels:read"]
}

# Custom function tools (user-defined Python)
tool calculate_risk {
  type        = "function"
  source      = "./tools/risk.py"
  description = "Calculate risk score for a given portfolio"
  parameters  = {
    portfolio_id = "string"
    timeframe    = "string"
  }
}
```

Tool types:
- **builtin** — shipped with Orca runtime (web_search, file_read, code_exec, etc.)
- **api** — external API integrations (slack, gmail, notion, jira, github, etc.)
- **function** — user-provided Python functions for custom logic

## Trigger subtypes

Use dot notation for trigger variants:

```hcl
trigger.cron daily {
  schedule = "0 9 * * *"
  starts   = workflow.research_pipeline
}

trigger.webhook on_push {
  endpoint = "/hooks/on-push"
  starts   = workflow.ci_pipeline
}

trigger.event on_complete {
  watches = task.summarize
  starts  = workflow.notify_pipeline
}
```

## Full example

```hcl
model gpt4 {
  provider    = "openai"
  version     = "gpt-4o"
  temperature = 0.2
}

model claude {
  provider    = "anthropic"
  version     = "claude-sonnet-4-20250514"
  temperature = 0.3
}

tool web_search {
  type = "builtin"
}

tool slack {
  type   = "api"
  scopes = ["chat:write"]
}

agent researcher {
  model  = gpt4
  tools  = [web_search]
  prompt = "You research topics thoroughly and return structured findings."
}

agent writer {
  model = claude
  prompt = <<EOF
    You are a technical writer.
    Turn research findings into clear, well-structured reports.
  EOF
}

agent reviewer {
  model  = gpt4
  prompt = "You review documents for accuracy and clarity. Return feedback or approve."
}

task research {
  description     = "Research the given topic"
  assignee        = researcher
  expected_output = "json"
}

task write_report {
  description     = "Write a report from the research"
  assignee        = writer
  expected_output = "markdown"
}

task review_report {
  description     = "Review the report for quality"
  assignee        = reviewer
  expected_output = "markdown"
}

workflow report_pipeline {
  researcher -> writer -> reviewer

  on_error = retry(2)
}

trigger.cron daily_report {
  schedule = "0 9 * * 1-5"
  starts   = workflow.report_pipeline
}
```

## Comparison: Orca vs raw LangGraph

The above ~70 lines of Orca replaces roughly 300+ lines of Python LangGraph code including:
- State class definitions
- Node function definitions with tool binding
- Graph construction with `add_node`, `add_edge`
- Conditional edge routing
- Tool executor setup
- Entry point configuration

Orca users declare *what* they want. The compiler handles *how*.

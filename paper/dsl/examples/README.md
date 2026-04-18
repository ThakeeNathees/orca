# Evaluation examples

Four use cases implemented in four stacks (Orca, Python+LangGraph,
CrewAI, and Docker cagent YAML). Used in the Evaluation section of
`paper/dsl/main.tex` for the lines-of-code comparison.

Physical SLOC reported in the paper is measured with:

```
# skip blank lines and pure-comment lines
awk 'NF && $0 !~ /^[[:space:]]*(#|\/\/)/' <file>
```

Use cases:

- `uc1-assistant/` — single-agent assistant.
- `uc2-research-writer/` — two-agent sequential pipeline.
- `uc3-scheduled-tool/` — tool-using agent with a scheduled trigger.
- `uc4-typed-multiagent/` — multi-agent workflow with a user-defined
  typed input/output schema.

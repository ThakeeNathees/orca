# TODO

## Next up

1. **Better error messages** — add context to parser errors ("in model block", show the offending line). Makes LSP diagnostics actually useful.

2. **Analyzer** — resolve references (does `model = gpt4` point to a defined block?), report undefined references, duplicate block names.

3. **LSP features** — go-to-definition, hover, completion. Depends on analyzer.

## Expressions

- Member access (`a.b`)
- Call expressions (`retry(3)`)
- Map literals (`{ key: value }`)
- Subscription (`a[0]`)

## Codegen

- Generate Python/LangGraph from AST
- Source mapping annotations in generated code

## Future

- `orca run` — build and execute generated Python
- Heredoc strings (`<<EOF ... EOF`) for multi-line prompts
- Conditional workflow branches
- Trigger subtype syntax (deferred)
- LSP semantic tokens (richer highlighting than TextMate)
- Publish VS Code extension to marketplace

# TODO

## LSP — Completion

- [ ] Complete field values after `=` based on field type (block refs, booleans, type names)
- [ ] Complete members after `.` (e.g. `gpt4.` suggests `provider`, `model_name`, `temperature`)
- [ ] Complete `@suppress("` with valid diagnostic codes (`undefined-ref`, `type-mismatch`, etc.)
- [ ] Complete annotation names after `@` (`desc`, `suppress`, `sensitive`)
- [ ] Complete block references in list context (e.g. `tools = [` suggests defined tool blocks)
- [ ] Complete type names in schema field values (e.g. `field = ` suggests `str`, `int`, `bool`, etc.)

## LSP — Diagnostics

- [ ] Validate annotation names (report unknown annotations)
- [ ] Validate `@desc` has exactly one string argument
- [ ] Validate `@suppress` arguments are valid diagnostic codes
- [ ] Warn on `field = null` (bare null type — probably unintended)
- [ ] Recurse into list elements for reference checking (`tools = [nonexistent]`)
- [ ] Cross-file diagnostics (multi-file projects)

## LSP — Other

- [ ] Document symbols (outline view / breadcrumbs)
- [ ] Rename symbol (rename a block and update all references)
- [ ] Find all references
- [ ] Signature help for annotations (`@desc(` shows parameter info)
- [ ] Semantic tokens (richer highlighting than TextMate grammar)
- [ ] Workspace symbols (search across files)

## Analyzer

- [ ] Validate user-defined schema block fields are valid type expressions
- [ ] Validate `input.default` value matches the declared `type`
- [ ] Detect unused blocks (defined but never referenced)
- [ ] Detect circular references in workflows
- [ ] Validate workflow arrow expressions (agents on both sides)

## Codegen

- [ ] Implement Python/LangGraph code generation
- [ ] Generate from `input` blocks (CLI args, env vars, or config)
- [ ] Generate from `schema` blocks (Pydantic models or dataclasses)
- [ ] Source mapping annotations in generated code

## CLI

- [ ] `orca fmt` — auto-format `.oc` files
- [ ] `orca check` — run diagnostics without building
- [ ] `orca init` — scaffold a new project
- [ ] Colored diagnostic output with codes (e.g. `[undefined-ref]`)

## Language

- [ ] `output` block (define what a task/workflow returns)
- [ ] `env` block or syntax for environment variable references
- [ ] Conditional fields or expressions
- [ ] Import system (`import "other.oc"`)
- [ ] String interpolation (`"Hello ${name}"`)

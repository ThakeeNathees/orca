# Implementation Checklist

Tracks what's implemented across the compiler pipeline.

## Statements (top-level blocks)

| Statement   | Lexer | Parser | AST Node       | Analyzer | Codegen | Tests |
|-------------|-------|--------|----------------|----------|---------|-------|
| `model`     | [x]   | [x]    | BlockStatement | [x]      | [ ]     | [x]   |
| `agent`     | [x]   | [x]    | BlockStatement | [x]      | [ ]     | [x]   |
| `tool`      | [x]   | [x]    | BlockStatement | [x]      | [ ]     | [x]   |
| `task`      | [x]   | [x]    | BlockStatement | [x]      | [ ]     | [x]   |
| `knowledge` | [x]   | [x]    | BlockStatement | [x]      | [ ]     | [x]   |
| `trigger`   | [x]   | [x]    | BlockStatement | [x]      | [ ]     | [x]   |
| `workflow`  | [x]   | [x]    | BlockStatement | [x]      | [ ]     | [x]   |
| `schema`    | [x]   | [x]    | BlockStatement | —        | —       | [x]   |

## Expressions (values in assignments)

| Expression       | Lexer | Parser | AST Node       | ExprType | Tests |
|------------------|-------|--------|----------------|----------|-------|
| Boolean literal  | [x]   | [x]    | BooleanLiteral | [x]      | [x]   |
| String literal   | [x]   | [x]    | StringLiteral  | [x]      | [x]   |
| Integer literal  | [x]   | [x]    | IntegerLiteral | [x]      | [x]   |
| Float literal    | [x]   | [x]    | FloatLiteral   | [x]      | [x]   |
| Identifier (ref) | [x]   | [x]    | Identifier     | [ ]      | [x]   |
| List literal     | [x]   | [x]    | ListLiteral    | [x]      | [x]   |
| Map literal      | [x]   | [x]    | MapLiteral     | [x]      | [x]   |
| Member access    | [x]   | [x]    | MemberAccess   | [ ]      | [x]   |
| Subscription     | [x]   | [x]    | Subscription   | [ ]      | [x]   |
| Call expression  | [x]   | [x]    | CallExpression | [ ]      | [x]   |

## Type system

| Feature                          | Status |
|----------------------------------|--------|
| Primitive types (str, int, etc.) | Done   |
| List[T] / Map[T]                | Done   |
| Union types (str \| model)       | Done   |
| Block reference types            | Done   |
| Schema definitions (block_schemas.oc) | Done |
| ExprType for literals            | Done   |
| ExprType for list/map inference  | Done   |
| Symbol table (block name → type) | TODO   |
| Identifier resolution            | TODO   |
| Member access resolution         | TODO   |

## Analyzer

| Feature                           | Status |
|-----------------------------------|--------|
| Missing required field detection  | Done   |
| Field type mismatch (literals)    | Done   |
| Unknown field name detection      | TODO   |
| Block reference resolution        | TODO — needs symbol table |
| Member access type resolution     | TODO — needs identifier resolution |
| Duplicate block name detection    | TODO   |
| Duplicate field name detection    | TODO   |

### Block reference resolution (TODO)

To resolve identifiers like `gpt4` and member access like `gpt4.model_name`:

1. **Symbol table**: Build a map of block names → block types by walking all `BlockStatement`s before analyzing assignments. e.g. `{"gpt4": BlockRef(model), "researcher": BlockRef(agent)}`

2. **Identifier resolution**: `ExprType` for `Identifier` looks up the name in the symbol table. If found, returns `BlockRef(model)`. If not found, report "undefined reference" error.

3. **Member access resolution**: Given the object's resolved type (e.g. `BlockRef(model)`), look up the field name in that block's schema to get the field type. e.g. `gpt4.model_name` → schema["model"]["model_name"] → `str | model`.

## LSP

| Feature                     | Status |
|-----------------------------|--------|
| Diagnostics (parse errors)  | Done   |
| Diagnostics (analyzer)      | Done   |
| Field name completion       | Done   |
| Field value completion      | TODO   |
| Cursor context resolution   | Done (cursor package) |
| Error-tolerant parsing      | Done   |
| Hover (field type/desc)     | TODO   |
| Go to definition            | TODO — needs symbol table |

## Workflow-specific syntax

| Feature              | Lexer | Parser | AST Node | Tests |
|----------------------|-------|--------|----------|-------|
| Arrow operator (`->`)| [x]   | [x]    | BinaryExpression | [x] |
| Linear flow          | [x]   | [x]    | BinaryExpression | [x] |
| Conditional branches | [ ]   | [ ]    | —        | [ ]   |

## Compiler pipeline

| Stage                     | Status      |
|---------------------------|-------------|
| Lexer                     | Done        |
| Parser (blocks + values)  | Done        |
| Parser (error recovery)   | Done        |
| AST                       | Done        |
| Type system               | Done (literals, schemas, unions) |
| Analyzer                  | Partial (required fields, type mismatch) |
| Python codegen (LangGraph)| Not started |
| CLI (`orca build`)        | Not started |
| CLI (`orca run`)          | Not started |
| Source mapping             | Tokens carry line/col + EndLine/EndCol |

## Error handling

| Feature                  | Status      |
|--------------------------|-------------|
| Lexer errors (ILLEGAL)   | Done        |
| Parser errors (syntax)   | Done        |
| Error recovery           | Done (sync to next block/assignment) |
| Missing required fields  | Done        |
| Type mismatch (literals) | Done        |
| Undefined references     | TODO — needs symbol table |
| Unknown field names      | TODO        |

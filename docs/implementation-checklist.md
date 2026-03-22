# Implementation Checklist

Tracks what's implemented across the compiler pipeline.

## Statements (top-level blocks)

| Statement   | Lexer | Parser | AST Node       | Evaluator/IR | Codegen | Tests |
|-------------|-------|--------|----------------|--------------|---------|-------|
| `model`     | [x]   | [x]    | BlockStatement | [ ]          | [ ]     | [x]   |
| `agent`     | [x]   | [x]    | BlockStatement | [ ]          | [ ]     | [x]   |
| `tool`      | [x]   | [x]    | BlockStatement | [ ]          | [ ]     | [x]   |
| `task`      | [x]   | [x]    | BlockStatement | [ ]          | [ ]     | [x]   |
| `knowledge` | [x]   | [x]    | BlockStatement | [ ]          | [ ]     | [x]   |
| `trigger`   | [x]   | [x]    | BlockStatement | [ ]          | [ ]     | [x]   |
| `workflow`  | [x]   | [x]    | BlockStatement | [ ]          | [ ]     | [x]   |

## Expressions (values in assignments)

| Expression       | Lexer | Parser | AST Node       | Evaluator/IR | Tests |
|------------------|-------|--------|----------------|--------------|-------|
| String literal   | [x]   | [x]    | StringLiteral  | [ ]          | [x]   |
| Integer literal  | [x]   | [x]    | IntegerLiteral | [ ]          | [x]   |
| Float literal    | [x]   | [x]    | FloatLiteral   | [ ]          | [x]   |
| Identifier (ref) | [x]   | [x]    | Identifier     | [ ]          | [x]   |
| List literal     | [x]   | [x]    | ListLiteral    | [ ]          | [x]   |
| Boolean literal  | [ ]   | [ ]    | —              | [ ]          | [ ]   |
| Dotted reference | [ ]   | [ ]    | —              | [ ]          | [ ]   |
| Heredoc string   | [ ]   | [ ]    | —              | [ ]          | [ ]   |

## Workflow-specific syntax

| Feature              | Lexer | Parser | AST Node | Evaluator/IR | Tests |
|----------------------|-------|--------|----------|--------------|-------|
| Arrow operator (`->`)| [ ]   | [ ]    | —        | [ ]          | [ ]   |
| Linear flow          | [ ]   | [ ]    | —        | [ ]          | [ ]   |
| Conditional branches | [ ]   | [ ]    | —        | [ ]          | [ ]   |

## Trigger subtypes

| Feature               | Lexer | Parser | AST Node | Evaluator/IR | Tests |
|-----------------------|-------|--------|----------|--------------|-------|
| `trigger.cron`        | [ ]   | [ ]    | —        | [ ]          | [ ]   |
| `trigger.webhook`     | [ ]   | [ ]    | —        | [ ]          | [ ]   |
| `trigger.event`       | [ ]   | [ ]    | —        | [ ]          | [ ]   |

## Compiler pipeline

| Stage                     | Status      |
|---------------------------|-------------|
| Lexer                     | Done        |
| Parser (blocks + values)  | Done        |
| Parser (workflow arrows)  | Not started |
| Parser (trigger subtypes) | Not started |
| AST                       | Done (blocks + values) |
| Evaluator / IR generation | Not started |
| Python codegen (LangGraph)| Not started |
| CLI (`orca build`)        | Not started |
| CLI (`orca run`)          | Not started |
| Source mapping             | Tokens carry line/col; codegen not started |

## Error handling

| Feature                  | Status      |
|--------------------------|-------------|
| Lexer errors (ILLEGAL)   | Done        |
| Parser errors (syntax)   | Done        |
| Semantic errors (undef)  | Not started |
| Type mismatch errors     | Not started |
| Error recovery           | Basic (skip token + continue) |

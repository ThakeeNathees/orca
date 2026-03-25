# Testing Strategy Proposal

## Approach: Table-driven tests

Go's table-driven test pattern fits Hive well — each compiler stage transforms structured input to structured output.

## By package

### Lexer tests

Test input strings against expected token sequences:

```go
func TestLexer(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected []token.Token
    }{
        {
            name:  "model block",
            input: `model gpt4 { provider = "openai" }`,
            expected: []token.Token{
                {Type: token.MODEL, Literal: "model", Line: 1, Column: 1},
                {Type: token.IDENT, Literal: "gpt4", Line: 1, Column: 7},
                // ...
            },
        },
    }
    // ...
}
```

### Parser tests

Parse input and assert AST structure. Use helper functions to reduce boilerplate:

```go
func TestParseModelBlock(t *testing.T) {
    input := `model gpt4 { provider = "openai" }`
    program := parseOrFail(t, input)
    assertBlockCount(t, program, 1)
    assertBlockType(t, program.Statements[0], "model", "gpt4")
}
```

### Evaluator / codegen tests

Test IR generation and Python output. Use golden files for codegen:

```
testdata/
  golden/
    simple_agent.oc          # input
    simple_agent.json         # expected IR
    simple_agent.py           # expected Python output
```

```go
func TestCodegen(t *testing.T) {
    input := readFile(t, "testdata/golden/simple_agent.oc")
    expected := readFile(t, "testdata/golden/simple_agent.py")
    got := compile(input)
    if got != expected {
        t.Errorf("codegen mismatch:\n%s", diff(expected, got))
    }
}
```

Update golden files with a flag: `go test -update-golden`.

### Integration tests

End-to-end: `.oc` file in, check that `build/` output is valid Python:

```go
func TestEndToEnd(t *testing.T) {
    // 1. Compile .oc to build/
    // 2. Run `python -c "import build.output"` to verify syntax
    // 3. Optionally run the generated code against a mock LLM
}
```

## Test helpers

Create a `testutil/` package (or keep in `_test.go` files):

- `parseOrFail(t, input)` — lex + parse, fail test on errors
- `assertNoErrors(t, parser)` — check parser error list is empty
- `assertBlockCount(t, program, n)` — check statement count
- `diff(expected, got)` — readable diff output

## Coverage targets

No hard coverage percentage targets. Focus on:
- Every token type has a lexer test
- Every AST node type has a parser test
- Every codegen pattern has a golden file
- Error cases: invalid syntax, undefined references, type mismatches

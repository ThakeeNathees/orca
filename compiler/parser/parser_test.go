package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/token"
)

func TestNew(t *testing.T) {
	l := lexer.New("")
	p := New(l)

	if p == nil {
		t.Fatal("expected parser, got nil")
	}
}

func TestParseProgramReturnsProgram(t *testing.T) {
	l := lexer.New("")
	p := New(l)
	program := p.ParseProgram()

	if program == nil {
		t.Fatal("ParseProgram() returned nil")
	}
	if len(program.Statements) != 0 {
		t.Errorf("expected 0 statements, got %d", len(program.Statements))
	}
}

func TestErrorsEmpty(t *testing.T) {
	l := lexer.New("")
	p := New(l)

	if len(p.Errors()) != 0 {
		t.Errorf("expected no errors, got %v", p.Errors())
	}
}

// --- helpers ---

func parseOrFail(t *testing.T, input string) *ast.Program {
	t.Helper()
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	if program == nil {
		t.Fatal("ParseProgram() returned nil")
	}
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	return program
}

func assertBlockCount(t *testing.T, program *ast.Program, expected int) {
	t.Helper()
	if len(program.Statements) != expected {
		t.Fatalf("expected %d statements, got %d", expected, len(program.Statements))
	}
}

func assertBlock(t *testing.T, stmt ast.Statement, expType token.TokenType, expName string) *ast.BlockStatement {
	t.Helper()
	block, ok := stmt.(*ast.BlockStatement)
	if !ok {
		t.Fatalf("expected *ast.BlockStatement, got %T", stmt)
	}
	if block.TokenStart.Type != expType {
		t.Errorf("expected block type %s, got %s", expType, block.TokenStart.Type)
	}
	if block.Name != expName {
		t.Errorf("expected block name %q, got %q", expName, block.Name)
	}
	return block
}

// --- model block tests ---

func TestParseModelBlock(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expName  string
		expCount int
	}{
		{
			name:     "model with string values",
			input:    `model gpt4 { provider = "openai" version = "gpt-4o" }`,
			expName:  "gpt4",
			expCount: 2,
		},
		{
			name:     "model with float value",
			input:    `model claude { provider = "anthropic" temperature = 0.3 }`,
			expName:  "claude",
			expCount: 2,
		},
		{
			name:     "model with only name",
			input:    `model minimal {}`,
			expName:  "minimal",
			expCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOrFail(t, tt.input)
			assertBlockCount(t, program, 1)
			block := assertBlock(t, program.Statements[0], token.MODEL, tt.expName)

			if len(block.Assignments) != tt.expCount {
				t.Errorf("expected %d assignments, got %d", tt.expCount, len(block.Assignments))
			}
		})
	}
}

func TestParseModelBlockValues(t *testing.T) {
	input := `model gpt4 {
		provider    = "openai"
		version     = "gpt-4o"
		temperature = 0.2
	}`

	program := parseOrFail(t, input)
	assertBlockCount(t, program, 1)
	block := assertBlock(t, program.Statements[0], token.MODEL, "gpt4")

	tests := []struct {
		key      string
		expType  string
		expValue interface{}
	}{
		{"provider", "string", "openai"},
		{"version", "string", "gpt-4o"},
		{"temperature", "float", 0.2},
	}

	for i, tt := range tests {
		a := block.Assignments[i]
		if a.Name != tt.key {
			t.Errorf("assignment %d: expected key %q, got %q", i, tt.key, a.Name)
		}
		switch tt.expType {
		case "string":
			s, ok := a.Value.(*ast.StringLiteral)
			if !ok {
				t.Errorf("assignment %d: expected StringLiteral, got %T", i, a.Value)
				continue
			}
			if s.Value != tt.expValue.(string) {
				t.Errorf("assignment %d: expected %q, got %q", i, tt.expValue, s.Value)
			}
		case "float":
			f, ok := a.Value.(*ast.FloatLiteral)
			if !ok {
				t.Errorf("assignment %d: expected FloatLiteral, got %T", i, a.Value)
				continue
			}
			if f.Value != tt.expValue.(float64) {
				t.Errorf("assignment %d: expected %v, got %v", i, tt.expValue, f.Value)
			}
		}
	}
}

// --- agent block tests ---

func TestParseAgentBlock(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expName  string
		expCount int
	}{
		{
			name:     "agent with reference and string",
			input:    `agent researcher { model = gpt4 prompt = "You research." }`,
			expName:  "researcher",
			expCount: 2,
		},
		{
			name:     "agent with list of tools",
			input:    `agent writer { model = claude tools = [web_search, gmail] prompt = "Write." }`,
			expName:  "writer",
			expCount: 3,
		},
		{
			name:     "empty agent",
			input:    `agent empty {}`,
			expName:  "empty",
			expCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOrFail(t, tt.input)
			assertBlockCount(t, program, 1)
			block := assertBlock(t, program.Statements[0], token.AGENT, tt.expName)

			if len(block.Assignments) != tt.expCount {
				t.Errorf("expected %d assignments, got %d", tt.expCount, len(block.Assignments))
			}
		})
	}
}

func TestParseAgentBlockValues(t *testing.T) {
	input := `agent researcher {
		model  = gpt4
		tools  = [web_search, gmail]
		prompt = "You are a research assistant."
	}`

	program := parseOrFail(t, input)
	assertBlockCount(t, program, 1)
	block := assertBlock(t, program.Statements[0], token.AGENT, "researcher")

	// model = gpt4 (reference)
	a0 := block.Assignments[0]
	if a0.Name != "model" {
		t.Errorf("expected key 'model', got %q", a0.Name)
	}
	ref, ok := a0.Value.(*ast.Identifier)
	if !ok {
		t.Fatalf("expected Identifier, got %T", a0.Value)
	}
	if ref.Value != "gpt4" {
		t.Errorf("expected 'gpt4', got %q", ref.Value)
	}

	// tools = [web_search, gmail] (list of references)
	a1 := block.Assignments[1]
	if a1.Name != "tools" {
		t.Errorf("expected key 'tools', got %q", a1.Name)
	}
	list, ok := a1.Value.(*ast.ListLiteral)
	if !ok {
		t.Fatalf("expected ListLiteral, got %T", a1.Value)
	}
	if len(list.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(list.Elements))
	}
	elem0, ok := list.Elements[0].(*ast.Identifier)
	if !ok {
		t.Fatalf("expected Identifier, got %T", list.Elements[0])
	}
	if elem0.Value != "web_search" {
		t.Errorf("expected 'web_search', got %q", elem0.Value)
	}
	elem1, ok := list.Elements[1].(*ast.Identifier)
	if !ok {
		t.Fatalf("expected Identifier, got %T", list.Elements[1])
	}
	if elem1.Value != "gmail" {
		t.Errorf("expected 'gmail', got %q", elem1.Value)
	}

	// prompt = "You are a research assistant." (string)
	a2 := block.Assignments[2]
	if a2.Name != "prompt" {
		t.Errorf("expected key 'prompt', got %q", a2.Name)
	}
	str, ok := a2.Value.(*ast.StringLiteral)
	if !ok {
		t.Fatalf("expected StringLiteral, got %T", a2.Value)
	}
	if str.Value != "You are a research assistant." {
		t.Errorf("expected prompt string, got %q", str.Value)
	}
}

// --- multiple blocks ---

func TestParseMultipleBlocks(t *testing.T) {
	input := `
model gpt4 {
	provider    = "openai"
	temperature = 0.2
}

agent researcher {
	model  = gpt4
	prompt = "Research things."
}
`
	program := parseOrFail(t, input)
	assertBlockCount(t, program, 2)
	assertBlock(t, program.Statements[0], token.MODEL, "gpt4")
	assertBlock(t, program.Statements[1], token.AGENT, "researcher")
}

// --- error cases ---

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing block name", `model { provider = "openai" }`},
		{"missing opening brace", `model gpt4 provider = "openai" }`},
		{"missing closing brace", `model gpt4 { provider = "openai"`},
		{"missing equals in assignment", `model gpt4 { provider "openai" }`},
		{"missing value after equals", `model gpt4 { provider = }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Error("expected parser errors, got none")
			}
		})
	}
}

// TestErrorRecoveryPartialAST verifies that the parser produces a partial
// AST for valid blocks even when other parts of the input have errors.
func TestErrorRecoveryPartialAST(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedBlocks int
		blockNames     []string
	}{
		{
			"valid block after broken block",
			"model { }\nagent researcher { persona = \"hi\" }",
			1,
			[]string{"researcher"},
		},
		{
			"missing closing brace still produces block",
			"model gpt4 { provider = \"openai\"",
			1,
			[]string{"gpt4"},
		},
		{
			"bad assignment recovers to next assignment",
			"model gpt4 { provider ! \"openai\"\n  temperature = 0.7 }",
			1,
			[]string{"gpt4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			program := p.ParseProgram()

			if !program.HasErrors {
				t.Error("expected HasErrors to be true")
			}
			if len(program.Statements) != tt.expectedBlocks {
				t.Fatalf("expected %d blocks, got %d", tt.expectedBlocks, len(program.Statements))
			}
			for i, name := range tt.blockNames {
				block, ok := program.Statements[i].(*ast.BlockStatement)
				if !ok {
					t.Fatalf("statement %d is not a BlockStatement", i)
				}
				if block.Name != name {
					t.Errorf("block %d name = %q, want %q", i, block.Name, name)
				}
			}
		})
	}
}

// --- integer value ---

func TestParseIntegerValue(t *testing.T) {
	input := `model m { max_tokens = 4096 }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.MODEL, "m")

	if len(block.Assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(block.Assignments))
	}
	intVal, ok := block.Assignments[0].Value.(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("expected IntegerLiteral, got %T", block.Assignments[0].Value)
	}
	if intVal.Value != 4096 {
		t.Errorf("expected 4096, got %d", intVal.Value)
	}
}

// --- list with strings ---

func TestParseListWithStrings(t *testing.T) {
	input := `agent a { scopes = ["read", "write"] }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.AGENT, "a")

	list, ok := block.Assignments[0].Value.(*ast.ListLiteral)
	if !ok {
		t.Fatalf("expected ListLiteral, got %T", block.Assignments[0].Value)
	}
	if len(list.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(list.Elements))
	}
	s0, ok := list.Elements[0].(*ast.StringLiteral)
	if !ok {
		t.Fatalf("expected StringLiteral, got %T", list.Elements[0])
	}
	if s0.Value != "read" {
		t.Errorf("expected 'read', got %q", s0.Value)
	}
}

// --- trailing commas ---

func TestParseListTrailingComma(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		numElems int
	}{
		{"single element trailing comma", `agent a { tools = [web_search,] }`, 1},
		{"two elements trailing comma", `agent a { scopes = ["read", "write",] }`, 2},
		{"three elements trailing comma", `agent a { tools = [a, b, c,] }`, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOrFail(t, tt.input)
			block := assertBlock(t, program.Statements[0], token.AGENT, "a")
			list, ok := block.Assignments[0].Value.(*ast.ListLiteral)
			if !ok {
				t.Fatalf("expected ListLiteral, got %T", block.Assignments[0].Value)
			}
			if len(list.Elements) != tt.numElems {
				t.Errorf("expected %d elements, got %d", tt.numElems, len(list.Elements))
			}
		})
	}
}

func TestParseMapTrailingComma(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		numEntries int
	}{
		{"single entry trailing comma", `model m { meta = {key: "val",} }`, 1},
		{"two entries trailing comma", `model m { meta = {a: 1, b: 2,} }`, 2},
		{"three entries trailing comma", `model m { meta = {a: 1, b: 2, c: 3,} }`, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOrFail(t, tt.input)
			block := assertBlock(t, program.Statements[0], token.MODEL, "m")
			mapLit, ok := block.Assignments[0].Value.(*ast.MapLiteral)
			if !ok {
				t.Fatalf("expected MapLiteral, got %T", block.Assignments[0].Value)
			}
			if len(mapLit.Entries) != tt.numEntries {
				t.Errorf("expected %d entries, got %d", tt.numEntries, len(mapLit.Entries))
			}
		})
	}
}

// --- keyword as assignment key ---

func TestParseKeywordAsAssignmentKey(t *testing.T) {
	input := `agent researcher { model = gpt4 }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.AGENT, "researcher")

	if len(block.Assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(block.Assignments))
	}
	if block.Assignments[0].Name != "model" {
		t.Errorf("expected key 'model', got %q", block.Assignments[0].Name)
	}
}

// --- boolean values ---

func TestParseBooleanValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"true value", `model m { verbose = true }`, true},
		{"false value", `model m { verbose = false }`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOrFail(t, tt.input)
			block := assertBlock(t, program.Statements[0], token.MODEL, "m")
			if len(block.Assignments) != 1 {
				t.Fatalf("expected 1 assignment, got %d", len(block.Assignments))
			}
			b, ok := block.Assignments[0].Value.(*ast.BooleanLiteral)
			if !ok {
				t.Fatalf("expected BooleanLiteral, got %T", block.Assignments[0].Value)
			}
			if b.Value != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, b.Value)
			}
		})
	}
}

// --- binary expressions ---

// exprString returns a parenthesized string representation of an expression
// to make precedence visible. E.g., `a + b * c` becomes `(a + (b * c))`.
func exprString(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Value
	case *ast.IntegerLiteral:
		return fmt.Sprintf("%d", e.Value)
	case *ast.FloatLiteral:
		return fmt.Sprintf("%g", e.Value)
	case *ast.StringLiteral:
		return fmt.Sprintf("%q", e.Value)
	case *ast.BooleanLiteral:
		if e.Value {
			return "true"
		}
		return "false"
	case *ast.BinaryExpression:
		return fmt.Sprintf("(%s %s %s)", exprString(e.Left), e.Operator.Literal, exprString(e.Right))
	case *ast.MemberAccess:
		return fmt.Sprintf("(%s.%s)", exprString(e.Object), e.Member)
	case *ast.Subscription:
		return fmt.Sprintf("(%s[%s])", exprString(e.Object), exprString(e.Index))
	case *ast.CallExpression:
		args := ""
		for i, a := range e.Arguments {
			if i > 0 {
				args += ", "
			}
			args += exprString(a)
		}
		return fmt.Sprintf("%s(%s)", exprString(e.Callee), args)
	case *ast.MapLiteral:
		entries := ""
		for i, entry := range e.Entries {
			if i > 0 {
				entries += ", "
			}
			entries += exprString(entry.Key) + ": " + exprString(entry.Value)
		}
		return fmt.Sprintf("{%s}", entries)
	default:
		return fmt.Sprintf("<%T>", expr)
	}
}

func TestParseBinaryExpression(t *testing.T) {
	input := `model m { val = 1 + 2 }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.MODEL, "m")

	be, ok := block.Assignments[0].Value.(*ast.BinaryExpression)
	if !ok {
		t.Fatalf("expected BinaryExpression, got %T", block.Assignments[0].Value)
	}
	if be.Operator.Literal != "+" {
		t.Errorf("expected operator '+', got %q", be.Operator.Literal)
	}
}

func TestParseOperatorPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "addition and multiplication",
			input:    `model m { val = a + b * c }`,
			expected: "(a + (b * c))",
		},
		{
			name:     "multiplication before subtraction",
			input:    `model m { val = a * b - c }`,
			expected: "((a * b) - c)",
		},
		{
			name:     "division and addition",
			input:    `model m { val = a / b + c }`,
			expected: "((a / b) + c)",
		},
		{
			name:     "same precedence left to right",
			input:    `model m { val = a + b - c }`,
			expected: "((a + b) - c)",
		},
		{
			name:     "same precedence mul/div left to right",
			input:    `model m { val = a * b / c }`,
			expected: "((a * b) / c)",
		},
		{
			name:     "arrow lowest precedence",
			input:    `model m { val = a + b -> c }`,
			expected: "((a + b) -> c)",
		},
		{
			name:     "arrow chains left to right",
			input:    `model m { val = a -> b -> c }`,
			expected: "((a -> b) -> c)",
		},
		{
			name:     "complex precedence",
			input:    `model m { val = a -> b + c * d }`,
			expected: "(a -> (b + (c * d)))",
		},
		{
			name:     "all operators",
			input:    `model m { val = a + b - c * d / e -> f }`,
			expected: "(((a + b) - ((c * d) / e)) -> f)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOrFail(t, tt.input)
			block := assertBlock(t, program.Statements[0], token.MODEL, "m")
			got := exprString(block.Assignments[0].Value)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

// --- member access ---

func TestParseMemberAccess(t *testing.T) {
	input := `model m { val = a.b }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.MODEL, "m")

	ma, ok := block.Assignments[0].Value.(*ast.MemberAccess)
	if !ok {
		t.Fatalf("expected MemberAccess, got %T", block.Assignments[0].Value)
	}
	ident, ok := ma.Object.(*ast.Identifier)
	if !ok {
		t.Fatalf("expected Identifier as object, got %T", ma.Object)
	}
	if ident.Value != "a" {
		t.Errorf("expected object 'a', got %q", ident.Value)
	}
	if ma.Member != "b" {
		t.Errorf("expected member 'b', got %q", ma.Member)
	}
}

func TestParseMemberAccessChained(t *testing.T) {
	input := `model m { val = a.b.c }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.MODEL, "m")

	got := exprString(block.Assignments[0].Value)
	expected := "((a.b).c)"
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestParseMemberAccessPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "member access binds tighter than addition",
			input:    `model m { val = a.b + c }`,
			expected: "((a.b) + c)",
		},
		{
			name:     "member access binds tighter than arrow",
			input:    `model m { val = a.b -> c.d }`,
			expected: "((a.b) -> (c.d))",
		},
		{
			name:     "member access binds tighter than multiplication",
			input:    `model m { val = a.b * c.d }`,
			expected: "((a.b) * (c.d))",
		},
		{
			name:     "chained member with arithmetic",
			input:    `model m { val = a.b.c + d }`,
			expected: "(((a.b).c) + d)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOrFail(t, tt.input)
			block := assertBlock(t, program.Statements[0], token.MODEL, "m")
			got := exprString(block.Assignments[0].Value)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestParseMemberAccessErrorMissingMember(t *testing.T) {
	l := lexer.New(`model m { val = a. }`)
	p := New(l)
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Error("expected error for missing member after dot")
	}
}

// --- subscription ---

func TestParseSubscription(t *testing.T) {
	input := `model m { val = a[0] }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.MODEL, "m")

	sub, ok := block.Assignments[0].Value.(*ast.Subscription)
	if !ok {
		t.Fatalf("expected Subscription, got %T", block.Assignments[0].Value)
	}
	ident, ok := sub.Object.(*ast.Identifier)
	if !ok {
		t.Fatalf("expected Identifier as object, got %T", sub.Object)
	}
	if ident.Value != "a" {
		t.Errorf("expected object 'a', got %q", ident.Value)
	}
	idx, ok := sub.Index.(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("expected IntegerLiteral as index, got %T", sub.Index)
	}
	if idx.Value != 0 {
		t.Errorf("expected index 0, got %d", idx.Value)
	}
}

func TestParseSubscriptionWithExpression(t *testing.T) {
	input := `model m { val = a[b + 1] }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.MODEL, "m")

	got := exprString(block.Assignments[0].Value)
	expected := "(a[(b + 1)])"
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestParseSubscriptionPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "subscription binds tighter than addition",
			input:    `model m { val = a[0] + b }`,
			expected: "((a[0]) + b)",
		},
		{
			name:     "subscription binds tighter than arrow",
			input:    `model m { val = a[0] -> b[1] }`,
			expected: "((a[0]) -> (b[1]))",
		},
		{
			name:     "chained subscription",
			input:    `model m { val = a[0][1] }`,
			expected: "((a[0])[1])",
		},
		{
			name:     "member access then subscription",
			input:    `model m { val = a.b[0] }`,
			expected: "((a.b)[0])",
		},
		{
			name:     "subscription then member access",
			input:    `model m { val = a[0].b }`,
			expected: "((a[0]).b)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOrFail(t, tt.input)
			block := assertBlock(t, program.Statements[0], token.MODEL, "m")
			got := exprString(block.Assignments[0].Value)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestParseSubscriptionErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing closing bracket", `model m { val = a[0 }`},
		{"empty subscript", `model m { val = a[] }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Error("expected parser errors, got none")
			}
		})
	}
}

// --- map literal ---

func TestParseMapLiteral(t *testing.T) {
	input := `model m { val = {name: "alice", age: 30} }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.MODEL, "m")

	ml, ok := block.Assignments[0].Value.(*ast.MapLiteral)
	if !ok {
		t.Fatalf("expected MapLiteral, got %T", block.Assignments[0].Value)
	}
	if len(ml.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(ml.Entries))
	}

	// First entry: name: "alice"
	key0, ok := ml.Entries[0].Key.(*ast.Identifier)
	if !ok {
		t.Fatalf("expected Identifier key, got %T", ml.Entries[0].Key)
	}
	if key0.Value != "name" {
		t.Errorf("expected key 'name', got %q", key0.Value)
	}
	val0, ok := ml.Entries[0].Value.(*ast.StringLiteral)
	if !ok {
		t.Fatalf("expected StringLiteral value, got %T", ml.Entries[0].Value)
	}
	if val0.Value != "alice" {
		t.Errorf("expected value 'alice', got %q", val0.Value)
	}

	// Second entry: age: 30
	key1, ok := ml.Entries[1].Key.(*ast.Identifier)
	if !ok {
		t.Fatalf("expected Identifier key, got %T", ml.Entries[1].Key)
	}
	if key1.Value != "age" {
		t.Errorf("expected key 'age', got %q", key1.Value)
	}
	val1, ok := ml.Entries[1].Value.(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("expected IntegerLiteral value, got %T", ml.Entries[1].Value)
	}
	if val1.Value != 30 {
		t.Errorf("expected value 30, got %d", val1.Value)
	}
}

func TestParseMapLiteralEmpty(t *testing.T) {
	input := `model m { val = {} }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.MODEL, "m")

	ml, ok := block.Assignments[0].Value.(*ast.MapLiteral)
	if !ok {
		t.Fatalf("expected MapLiteral, got %T", block.Assignments[0].Value)
	}
	if len(ml.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(ml.Entries))
	}
}

func TestParseMapLiteralStringKeys(t *testing.T) {
	input := `model m { val = {"key": "value"} }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.MODEL, "m")

	ml, ok := block.Assignments[0].Value.(*ast.MapLiteral)
	if !ok {
		t.Fatalf("expected MapLiteral, got %T", block.Assignments[0].Value)
	}
	if len(ml.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(ml.Entries))
	}
	key, ok := ml.Entries[0].Key.(*ast.StringLiteral)
	if !ok {
		t.Fatalf("expected StringLiteral key, got %T", ml.Entries[0].Key)
	}
	if key.Value != "key" {
		t.Errorf("expected key 'key', got %q", key.Value)
	}
}

func TestParseMapLiteralExprString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single entry",
			input:    `model m { val = {a: 1} }`,
			expected: "{a: 1}",
		},
		{
			name:     "multiple entries",
			input:    `model m { val = {a: 1, b: 2} }`,
			expected: "{a: 1, b: 2}",
		},
		{
			name:     "nested map",
			input:    `model m { val = {a: {b: 1}} }`,
			expected: "{a: {b: 1}}",
		},
		{
			name:     "map with expression values",
			input:    `model m { val = {a: x + 1} }`,
			expected: "{a: (x + 1)}",
		},
		{
			name:     "map in list",
			input:    `model m { val = [{a: 1}] }`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOrFail(t, tt.input)
			block := assertBlock(t, program.Statements[0], token.MODEL, "m")
			got := exprString(block.Assignments[0].Value)
			if tt.expected != "" && got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestParseMapLiteralErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing colon", `model m { val = {a 1} }`},
		{"missing value after colon", `model m { val = {a: } }`},
		{"missing closing brace", `model m { val = {a: 1 }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Error("expected parser errors, got none")
			}
		})
	}
}

// --- call expression ---

func TestParseCallExpression(t *testing.T) {
	input := `model m { val = retry(3) }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.MODEL, "m")

	call, ok := block.Assignments[0].Value.(*ast.CallExpression)
	if !ok {
		t.Fatalf("expected CallExpression, got %T", block.Assignments[0].Value)
	}
	callee, ok := call.Callee.(*ast.Identifier)
	if !ok {
		t.Fatalf("expected Identifier as callee, got %T", call.Callee)
	}
	if callee.Value != "retry" {
		t.Errorf("expected callee 'retry', got %q", callee.Value)
	}
	if len(call.Arguments) != 1 {
		t.Fatalf("expected 1 argument, got %d", len(call.Arguments))
	}
	arg, ok := call.Arguments[0].(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("expected IntegerLiteral as argument, got %T", call.Arguments[0])
	}
	if arg.Value != 3 {
		t.Errorf("expected argument 3, got %d", arg.Value)
	}
}

func TestParseCallExpressionNoArgs(t *testing.T) {
	input := `model m { val = reset() }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.MODEL, "m")

	call, ok := block.Assignments[0].Value.(*ast.CallExpression)
	if !ok {
		t.Fatalf("expected CallExpression, got %T", block.Assignments[0].Value)
	}
	if len(call.Arguments) != 0 {
		t.Errorf("expected 0 arguments, got %d", len(call.Arguments))
	}
}

func TestParseCallExpressionMultipleArgs(t *testing.T) {
	input := `model m { val = fallback(backup_agent, "default", 3) }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.MODEL, "m")

	call, ok := block.Assignments[0].Value.(*ast.CallExpression)
	if !ok {
		t.Fatalf("expected CallExpression, got %T", block.Assignments[0].Value)
	}
	if len(call.Arguments) != 3 {
		t.Fatalf("expected 3 arguments, got %d", len(call.Arguments))
	}
}

func TestParseCallExpressionPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "call binds tighter than addition",
			input:    `model m { val = foo(1) + 2 }`,
			expected: "(foo(1) + 2)",
		},
		{
			name:     "call binds tighter than arrow",
			input:    `model m { val = foo(1) -> bar(2) }`,
			expected: "(foo(1) -> bar(2))",
		},
		{
			name:     "member access then call",
			input:    `model m { val = a.b(1) }`,
			expected: "(a.b)(1)",
		},
		{
			name:     "chained calls",
			input:    `model m { val = foo(1)(2) }`,
			expected: "foo(1)(2)",
		},
		{
			name:     "call with expression argument",
			input:    `model m { val = foo(a + b) }`,
			expected: "foo((a + b))",
		},
		{
			name:     "call then subscription",
			input:    `model m { val = foo(1)[0] }`,
			expected: "(foo(1)[0])",
		},
		{
			name:     "subscription then call",
			input:    `model m { val = a[0](1) }`,
			expected: "(a[0])(1)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOrFail(t, tt.input)
			block := assertBlock(t, program.Statements[0], token.MODEL, "m")
			got := exprString(block.Assignments[0].Value)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestParseCallExpressionErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing closing paren", `model m { val = foo(1 }`},
		{"missing closing paren empty", `model m { val = foo( }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Error("expected parser errors, got none")
			}
		})
	}
}

// --- file-based tests ---

// readTestFile reads a .oc file from the testdata directory.
func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read test file %s: %v", path, err)
	}
	return string(data)
}

// TestValidFiles walks testdata/valid/ and verifies each .oc file parses
// without errors and produces at least one statement.
func TestValidFiles(t *testing.T) {
	files, err := filepath.Glob("testdata/valid/*.oc")
	if err != nil {
		t.Fatalf("failed to glob valid test files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no valid test files found in testdata/valid/")
	}

	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			input := readTestFile(t, file)
			l := lexer.New(input)
			p := New(l)
			program := p.ParseProgram()

			if len(p.Errors()) > 0 {
				t.Errorf("expected no errors for %s, got: %v", name, p.Errors())
			}
			if program == nil {
				t.Fatalf("ParseProgram() returned nil for %s", name)
			}
			if len(program.Statements) == 0 {
				t.Errorf("expected at least one statement in %s", name)
			}
		})
	}
}

// TestInvalidFiles walks testdata/invalid/ and verifies each .oc file
// produces at least one parse error.
func TestInvalidFiles(t *testing.T) {
	files, err := filepath.Glob("testdata/invalid/*.oc")
	if err != nil {
		t.Fatalf("failed to glob invalid test files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no invalid test files found in testdata/invalid/")
	}

	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			input := readTestFile(t, file)
			l := lexer.New(input)
			p := New(l)
			p.ParseProgram()

			if len(p.Errors()) == 0 {
				t.Errorf("expected parse errors for %s, got none", name)
			}
		})
	}
}

// TestParseNullLiteral verifies that null is parsed as a NullLiteral.
func TestParseNullLiteral(t *testing.T) {
	input := `input x { default = null }`
	program := parseOrFail(t, input)
	assertBlockCount(t, program, 1)
	block := assertBlock(t, program.Statements[0], token.INPUT, "x")
	if len(block.Assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(block.Assignments))
	}
	if _, ok := block.Assignments[0].Value.(*ast.NullLiteral); !ok {
		t.Errorf("expected NullLiteral, got %T", block.Assignments[0].Value)
	}
}

// TestParseNullInUnion verifies that str | null parses as a BinaryExpression.
func TestParseNullInUnion(t *testing.T) {
	input := `schema s { field = str | null }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.SCHEMA, "s")
	binExpr, ok := block.Assignments[0].Value.(*ast.BinaryExpression)
	if !ok {
		t.Fatalf("expected BinaryExpression, got %T", block.Assignments[0].Value)
	}
	if binExpr.Operator.Type != token.PIPE {
		t.Errorf("expected PIPE operator, got %s", binExpr.Operator.Type)
	}
	if _, ok := binExpr.Right.(*ast.NullLiteral); !ok {
		t.Errorf("expected NullLiteral on right, got %T", binExpr.Right)
	}
}

// TestParseFieldAnnotation verifies that annotations before assignments are parsed.
func TestParseFieldAnnotation(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		annName     string
		annArgCount int
		fieldName   string
	}{
		{
			"bare annotation",
			"schema s {\n  @required\n  region = str\n}",
			"required",
			0,
			"region",
		},
		{
			"annotation with string arg",
			"schema s {\n  @desc(\"AWS region\")\n  region = str\n}",
			"desc",
			1,
			"region",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOrFail(t, tt.input)
			block := assertBlock(t, program.Statements[0], token.SCHEMA, "s")
			if len(block.Assignments) != 1 {
				t.Fatalf("expected 1 assignment, got %d", len(block.Assignments))
			}
			assign := block.Assignments[0]
			if assign.Name != tt.fieldName {
				t.Errorf("field name = %q, want %q", assign.Name, tt.fieldName)
			}
			if len(assign.Annotations) != 1 {
				t.Fatalf("expected 1 annotation, got %d", len(assign.Annotations))
			}
			ann := assign.Annotations[0]
			if ann.Name != tt.annName {
				t.Errorf("annotation name = %q, want %q", ann.Name, tt.annName)
			}
			if len(ann.Arguments) != tt.annArgCount {
				t.Errorf("annotation args = %d, want %d", len(ann.Arguments), tt.annArgCount)
			}
		})
	}
}

// TestParseMultipleAnnotations verifies that multiple annotations
// on a single field are collected correctly.
func TestParseMultipleAnnotations(t *testing.T) {
	input := "schema s {\n  @required\n  @desc(\"region\")\n  region = str\n}"
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.SCHEMA, "s")
	assign := block.Assignments[0]
	if len(assign.Annotations) != 2 {
		t.Fatalf("expected 2 annotations, got %d", len(assign.Annotations))
	}
	if assign.Annotations[0].Name != "required" {
		t.Errorf("first annotation = %q, want %q", assign.Annotations[0].Name, "required")
	}
	if assign.Annotations[1].Name != "desc" {
		t.Errorf("second annotation = %q, want %q", assign.Annotations[1].Name, "desc")
	}
}

// TestParseBlockAnnotation verifies that annotations before a block keyword
// are attached to the BlockStatement.
func TestParseBlockAnnotation(t *testing.T) {
	input := "@sensitive\ninput apikey {\n  type = str\n}"
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.INPUT, "apikey")
	if len(block.Annotations) != 1 {
		t.Fatalf("expected 1 block annotation, got %d", len(block.Annotations))
	}
	if block.Annotations[0].Name != "sensitive" {
		t.Errorf("annotation name = %q, want %q", block.Annotations[0].Name, "sensitive")
	}
}

// TestParseNoAnnotations verifies that assignments without annotations
// have an empty annotations slice.
func TestParseNoAnnotations(t *testing.T) {
	input := `model m { provider = "openai" }`
	program := parseOrFail(t, input)
	block := assertBlock(t, program.Statements[0], token.MODEL, "m")
	if len(block.Assignments[0].Annotations) != 0 {
		t.Errorf("expected 0 annotations, got %d", len(block.Assignments[0].Annotations))
	}
	if len(block.Annotations) != 0 {
		t.Errorf("expected 0 block annotations, got %d", len(block.Annotations))
	}
}

// TestParseSchemaExpression verifies parsing of inline schema expressions.
func TestParseSchemaExpression(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		fieldCount int
		fieldNames []string
	}{
		{
			"inline schema with two fields",
			`input x {
				type = schema {
					key   = str
					model = str
				}
			}`,
			2,
			[]string{"key", "model"},
		},
		{
			"empty inline schema",
			`input x {
				type = schema {}
			}`,
			0,
			nil,
		},
		{
			"inline schema with complex types",
			`agent a {
				model   = "gpt4"
				persona = "test"
				output = schema {
					draft    = str
					revision = int
					tags     = list[str]
				}
			}`,
			3,
			[]string{"draft", "revision", "tags"},
		},
		{
			"schema keyword without brace is identifier",
			`input x {
				type = schema
			}`,
			-1, // not a schema expression, just an identifier
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOrFail(t, tt.input)
			block := program.Statements[0].(*ast.BlockStatement)

			// Find the assignment with the schema expression.
			var target *ast.Assignment
			for _, a := range block.Assignments {
				if a.Name == "type" || a.Name == "output" {
					target = a
					break
				}
			}
			if target == nil {
				t.Fatal("target assignment not found")
			}

			if tt.fieldCount == -1 {
				// Expect an identifier, not a schema expression.
				if _, ok := target.Value.(*ast.Identifier); !ok {
					t.Fatalf("expected *ast.Identifier, got %T", target.Value)
				}
				return
			}

			se, ok := target.Value.(*ast.SchemaExpression)
			if !ok {
				t.Fatalf("expected *ast.SchemaExpression, got %T", target.Value)
			}
			if len(se.Assignments) != tt.fieldCount {
				t.Fatalf("expected %d fields, got %d", tt.fieldCount, len(se.Assignments))
			}
			for i, name := range tt.fieldNames {
				if se.Assignments[i].Name != name {
					t.Errorf("field[%d] name = %q, want %q", i, se.Assignments[i].Name, name)
				}
			}
		})
	}
}

// TestParseSchemaExpressionErrors verifies error cases for inline schema parsing.
func TestParseSchemaExpressionErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			"missing closing brace",
			`input x { type = schema { key = str }`,
		},
		{
			"missing value after equals",
			`input x { type = schema { key = } }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Error("expected parse errors, got none")
			}
		})
	}
}

// --- let block tests ---

func TestParseLetBlock(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expCount int // number of assignments
		expKeys  []string
	}{
		{
			name:     "let with string values",
			input:    `let { name = "hello" greeting = "world" }`,
			expCount: 2,
			expKeys:  []string{"name", "greeting"},
		},
		{
			name:     "let with mixed types",
			input:    `let { api_url = "https://example.com" max_retries = 3 temp = 0.7 debug = true }`,
			expCount: 4,
			expKeys:  []string{"api_url", "max_retries", "temp", "debug"},
		},
		{
			name:     "empty let block",
			input:    `let {}`,
			expCount: 0,
			expKeys:  nil,
		},
		{
			name:     "multiple let blocks",
			input:    `let { a = "1" } let { b = "2" }`,
			expCount: 1, // per block
			expKeys:  []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOrFail(t, tt.input)
			if len(program.Statements) == 0 {
				t.Fatal("expected at least one statement")
			}
			block := assertBlock(t, program.Statements[0], token.LET, "")
			if len(block.Assignments) != tt.expCount {
				t.Fatalf("expected %d assignments, got %d", tt.expCount, len(block.Assignments))
			}
			for i, key := range tt.expKeys {
				if block.Assignments[i].Name != key {
					t.Errorf("assignment[%d] name = %q, want %q", i, block.Assignments[i].Name, key)
				}
			}
		})
	}
}

func TestParseLetBlockComplexValues(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			"member access",
			`let { val = foo.bar }`,
		},
		{
			"subscription",
			`let { val = foo["bar"] }`,
		},
		{
			"chained access",
			`let { val = foo["bar"].baz }`,
		},
		{
			"list value",
			`let { items = [1, 2, 3] }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOrFail(t, tt.input)
			block := assertBlock(t, program.Statements[0], token.LET, "")
			if len(block.Assignments) != 1 {
				t.Fatalf("expected 1 assignment, got %d", len(block.Assignments))
			}
		})
	}
}

func TestParseLetBlockErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			"let with name",
			`let myname { a = 1 }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Error("expected parse errors, got none")
			}
		})
	}
}

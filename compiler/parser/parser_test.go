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

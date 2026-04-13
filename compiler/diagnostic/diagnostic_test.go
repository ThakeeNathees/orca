package diagnostic

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

func TestDiagnosticError(t *testing.T) {
	tests := []struct {
		name     string
		diag     Diagnostic
		expected string
	}{
		{
			name: "parser error",
			diag: Diagnostic{
				Severity: Error,
				Code:     CodeSyntax,
				Position: Position{Line: 3, Column: 5},
				Message:  "expected }",
				Source:   "parser",
			},
			expected: "parser:3:5: [syntax] expected }",
		},
		{
			name: "analyzer warning",
			diag: Diagnostic{
				Severity: Warning,
				Code:     CodeUndefinedRef,
				Position: Position{Line: 1, Column: 1},
				Message:  "undefined reference 'gpt4'",
				Source:   "analyzer",
			},
			expected: "analyzer:1:1: [undefined-ref] undefined reference 'gpt4'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.diag.Error()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestPositionOf(t *testing.T) {
	tok := token.Token{Line: 3, Column: 7, Literal: "gpt4"}
	got := PositionOf(tok)
	want := Position{Line: 3, Column: 7}
	if got != want {
		t.Errorf("PositionOf = %+v, want %+v", got, want)
	}
}

func TestEndPositionOf(t *testing.T) {
	tests := []struct {
		name string
		tok  token.Token
		want Position
	}{
		{
			name: "explicit EndCol on same line (inclusive → exclusive)",
			tok:  token.Token{Line: 3, Column: 7, Literal: "gpt4", EndLine: 3, EndCol: 10},
			want: Position{Line: 3, Column: 11},
		},
		{
			name: "multi-line raw string uses EndLine",
			tok:  token.Token{Line: 2, Column: 5, Literal: "```x\ny\n```", EndLine: 4, EndCol: 3},
			want: Position{Line: 4, Column: 4},
		},
		{
			name: "fallback to Column + len(Literal)",
			tok:  token.Token{Line: 1, Column: 1, Literal: "model"},
			want: Position{Line: 1, Column: 6},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EndPositionOf(tt.tok)
			if got != tt.want {
				t.Errorf("EndPositionOf = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRangeOf(t *testing.T) {
	// Build a minimal AST node: an Identifier spanning one token.
	tok := token.Token{Type: token.IDENT, Literal: "gpt4", Line: 5, Column: 7, EndLine: 5, EndCol: 10}
	node := &ast.Identifier{BaseNode: ast.NewTerminal(tok), Value: "gpt4"}

	start, end := RangeOf(node)
	if start != (Position{Line: 5, Column: 7}) {
		t.Errorf("start = %+v", start)
	}
	if end != (Position{Line: 5, Column: 11}) {
		t.Errorf("end = %+v", end)
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		severity Severity
		expected string
	}{
		{Error, "error"},
		{Warning, "warning"},
		{Info, "info"},
		{Hint, "hint"},
		{Severity(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.severity.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.severity.String())
			}
		})
	}
}

package diagnostic

import (
	"testing"

	"github.com/fatih/color"
)

// TestRender verifies diagnostic.Render produces the expected multi-line
// snippet output. Colors are disabled so assertions are hermetic.
func TestRender(t *testing.T) {
	color.NoColor = true

	const src5 = "line one\nline two\nline three\nline four\nline five\n"

	tests := []struct {
		name     string
		source   string
		diag     Diagnostic
		expected string
	}{
		{
			name:   "middle line with single-token range",
			source: src5,
			diag: Diagnostic{
				Severity:    Error,
				Code:        CodeSyntax,
				Position:    Position{Line: 3, Column: 6},
				EndPosition: Position{Line: 3, Column: 10},
				Message:     "expected }",
				Source:      "parser",
				File:        "foo.orca",
			},
			expected: "" +
				"error[syntax]: expected }\n" +
				"  --> foo.orca:3:6\n" +
				"   |\n" +
				" 1 | line one\n" +
				" 2 | line two\n" +
				" 3 | line three\n" +
				"   |      ~~~~\n" +
				" 4 | line four\n" +
				" 5 | line five\n",
		},
		{
			name:   "top of file (no lines above)",
			source: src5,
			diag: Diagnostic{
				Severity: Error,
				Code:     CodeSyntax,
				Position: Position{Line: 1, Column: 1},
				Message:  "bad start",
				File:     "foo.orca",
			},
			expected: "" +
				"error[syntax]: bad start\n" +
				"  --> foo.orca:1:1\n" +
				"   |\n" +
				" 1 | line one\n" +
				"   | ~\n" +
				" 2 | line two\n" +
				" 3 | line three\n",
		},
		{
			name:   "bottom of file (no lines below)",
			source: src5,
			diag: Diagnostic{
				Severity: Error,
				Code:     CodeSyntax,
				Position: Position{Line: 5, Column: 1},
				Message:  "trailing",
				File:     "foo.orca",
			},
			expected: "" +
				"error[syntax]: trailing\n" +
				"  --> foo.orca:5:1\n" +
				"   |\n" +
				" 3 | line three\n" +
				" 4 | line four\n" +
				" 5 | line five\n" +
				"   | ~\n",
		},
		{
			name:   "multi-line range underlines first line only",
			source: src5,
			diag: Diagnostic{
				Severity:    Error,
				Code:        CodeSyntax,
				Position:    Position{Line: 2, Column: 6},
				EndPosition: Position{Line: 4, Column: 3},
				Message:     "unterminated block",
				File:        "foo.orca",
			},
			expected: "" +
				"error[syntax]: unterminated block\n" +
				"  --> foo.orca:2:6\n" +
				"   |\n" +
				" 1 | line one\n" +
				" 2 | line two\n" +
				"   |      ~~~\n" +
				" 3 | line three\n" +
				" 4 | line four\n",
		},
		{
			name:   "tab expansion aligns underline",
			source: "a\n\tfoo bar\nb\n",
			diag: Diagnostic{
				Severity:    Error,
				Code:        CodeSyntax,
				Position:    Position{Line: 2, Column: 2},
				EndPosition: Position{Line: 2, Column: 5},
				Message:     "bad token",
				File:        "foo.orca",
			},
			expected: "" +
				"error[syntax]: bad token\n" +
				"  --> foo.orca:2:2\n" +
				"   |\n" +
				" 1 | a\n" +
				" 2 |     foo bar\n" +
				"   |     ~~~\n" +
				" 3 | b\n",
		},
		{
			name:   "warning severity header",
			source: src5,
			diag: Diagnostic{
				Severity: Warning,
				Code:     CodeUndefinedRef,
				Position: Position{Line: 2, Column: 1},
				Message:  "unused",
				File:     "foo.orca",
			},
			expected: "" +
				"warning[undefined-ref]: unused\n" +
				"  --> foo.orca:2:1\n" +
				"   |\n" +
				" 1 | line one\n" +
				" 2 | line two\n" +
				"   | ~\n" +
				" 3 | line three\n" +
				" 4 | line four\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Render(tt.source, tt.diag)
			if got != tt.expected {
				t.Errorf("Render mismatch\n--- expected ---\n%s\n--- got ---\n%s", tt.expected, got)
			}
		})
	}
}

package diagnostic

import "testing"

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
				Position: Position{Line: 3, Column: 5},
				Message:  "expected }",
				Source:   "parser",
			},
			expected: "parser:3:5: expected }",
		},
		{
			name: "analyzer warning",
			diag: Diagnostic{
				Severity: Warning,
				Position: Position{Line: 1, Column: 1},
				Message:  "undefined reference 'gpt4'",
				Source:   "analyzer",
			},
			expected: "analyzer:1:1: undefined reference 'gpt4'",
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

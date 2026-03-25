package codegen

import "testing"

// TestSourceComment verifies source mapping comment formatting.
func TestSourceComment(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		line     int
		expected string
	}{
		{"with file", "agents.oc", 42, "agents.oc:42"},
		{"with file line 1", "main.oc", 1, "main.oc:1"},
		{"without file", "", 42, "line 42"},
		{"without file line 1", "", 1, "line 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SourceComment(tt.file, tt.line)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

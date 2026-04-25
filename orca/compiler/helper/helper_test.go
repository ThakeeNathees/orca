package helper

import "testing"

// TestToPascalCase verifies snake_case to PascalCase conversion.
func TestToPascalCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"single word", "article", "Article"},
		{"two words", "research_report", "ResearchReport"},
		{"three words", "vpc_data_t", "VpcDataT"},
		{"already pascal", "Article", "Article"},
		{"single char segments", "a_b_c", "ABC"},
		{"trailing underscore", "foo_", "Foo"},
		{"leading underscore", "_foo", "Foo"},
		{"double underscore", "foo__bar", "FooBar"},
		{"all caps segment", "api_key", "ApiKey"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToPascalCase(tt.input)
			if got != tt.expected {
				t.Errorf("ToPascalCase(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

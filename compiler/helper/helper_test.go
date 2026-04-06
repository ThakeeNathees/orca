package helper

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
)

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

// TestHasAnnotation verifies detection of a named decorator in an annotation list.
func TestHasAnnotation(t *testing.T) {
	tests := []struct {
		name        string
		annotations []*ast.Annotation
		wantName    string
		want        bool
	}{
		{
			name:        "nil slice",
			annotations: nil,
			wantName:    "only_assignment",
			want:        false,
		},
		{
			name:        "empty slice",
			annotations: []*ast.Annotation{},
			wantName:    "only_assignment",
			want:        false,
		},
		{
			name:        "matching name",
			annotations: []*ast.Annotation{{Name: "only_assignment"}},
			wantName:    "only_assignment",
			want:        true,
		},
		{
			name:        "different name",
			annotations: []*ast.Annotation{{Name: "other"}},
			wantName:    "only_assignment",
			want:        false,
		},
		{
			name:        "skips nil entries",
			annotations: []*ast.Annotation{nil, {Name: "only_assignment"}},
			wantName:    "only_assignment",
			want:        true,
		},
		{
			name:        "second match",
			annotations: []*ast.Annotation{{Name: "suppress"}, {Name: "only_assignment"}},
			wantName:    "only_assignment",
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasAnnotation(tt.annotations, tt.wantName)
			if got != tt.want {
				t.Errorf("HasAnnotation(...) = %v, want %v", got, tt.want)
			}
		})
	}
}

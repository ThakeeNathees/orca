package types

import (
	"testing"

	"github.com/thakee/orca/orca/compiler/ast"
)

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

package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefinitionNestedSchemaChain verifies go-to-definition along
// t → v → ls → item for a user-defined schema, instance block, and list-of-schema field.
// Loads compiler/lsp/testdata/definition_nested_schema.orca (1-based line/col in table match that file).
func TestDefinitionNestedSchemaChain(t *testing.T) {
	path := filepath.Join("testdata", "definition_nested_schema.orca")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	doc := updateDocument("test://definition_nested_schema.orca", string(src))
	if doc.Symbols == nil {
		t.Fatal("expected symbol table")
	}

	tests := []struct {
		name                 string
		line1, col1          int
		wantLine0, wantChar0 int
	}{
		// t in baz = t.v... → instance name my_t t {}
		{"t", 16, 9, 12, 5},
		// v → v = schema { ... } in schema my_t
		{"v", 16, 11, 2, 2},
		// ls → ls = list [ ... ]
		{"ls", 16, 13, 3, 4},
		// item → item = string in nested schema
		{"item", 16, 19, 5, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, found := resolveDefinition(doc, tt.line1, tt.col1)
			if !found {
				t.Fatal("expected definition location")
			}
			if int(loc.Range.Start.Line) != tt.wantLine0 || int(loc.Range.Start.Character) != tt.wantChar0 {
				t.Errorf("definition at (%d, %d), want (%d, %d)",
					loc.Range.Start.Line, loc.Range.Start.Character,
					tt.wantLine0, tt.wantChar0)
			}
		})
	}
}

package types

import (
	"testing"

	"github.com/thakee/orca/compiler/token"
)

// TestSymbolTableLookup verifies basic symbol table operations.
func TestSymbolTableLookup(t *testing.T) {
	st := NewSymbolTable()
	st.Define("gpt4", NewBlockRefType(BlockModel), token.Token{})
	st.Define("researcher", NewBlockRefType(BlockAgent), token.Token{})

	tests := []struct {
		name     string
		symbol   string
		found    bool
		expected TypeKind
	}{
		{"defined model", "gpt4", true, BlockRef},
		{"defined agent", "researcher", true, BlockRef},
		{"undefined", "unknown", false, BlockRef},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ, ok := st.Lookup(tt.symbol)
			if ok != tt.found {
				t.Errorf("Lookup(%q) found = %v, want %v", tt.symbol, ok, tt.found)
			}
			if ok && typ.Kind != tt.expected {
				t.Errorf("Lookup(%q) Kind = %v, want %v", tt.symbol, typ.Kind, tt.expected)
			}
		})
	}
}

// TestSymbolTableBlockType verifies that the BlockType is preserved.
func TestSymbolTableBlockType(t *testing.T) {
	st := NewSymbolTable()
	st.Define("gpt4", NewBlockRefType(BlockModel), token.Token{})

	typ, ok := st.Lookup("gpt4")
	if !ok {
		t.Fatal("expected gpt4 to be defined")
	}
	if typ.BlockType != BlockModel {
		t.Errorf("BlockType = %v, want %v", typ.BlockType, BlockModel)
	}
}

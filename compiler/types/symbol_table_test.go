package types

import (
	"testing"

	"github.com/thakee/orca/compiler/token"
)

// TestSymbolTableLookup verifies basic symbol table operations.
func TestSymbolTableLookup(t *testing.T) {
	st := NewSymbolTable()
	st.Define("gpt4", NewBlockRefType("gpt4", nil), token.Token{})
	st.Define("researcher", NewBlockRefType("researcher", nil), token.Token{})

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

// TestLookupSymbol verifies that LookupSymbol returns the full Symbol
// including the definition token, and returns false for undefined symbols.
func TestLookupSymbol(t *testing.T) {
	st := NewSymbolTable()
	defTok := token.Token{Type: token.IDENT, Literal: "mymodel", Line: 5, Column: 3}
	st.Define("mymodel", NewBlockRefType("mymodel", nil), defTok)

	tests := []struct {
		name            string
		symbol          string
		found           bool
		expectBlockName string
		expectLn        int
		expectCol       int
	}{
		{"defined symbol returns full data", "mymodel", true, "mymodel", 5, 3},
		{"undefined symbol returns false", "nosuchsym", false, "", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sym, ok := st.LookupSymbol(tt.symbol)
			if ok != tt.found {
				t.Errorf("LookupSymbol(%q) found = %v, want %v", tt.symbol, ok, tt.found)
			}
			if ok {
				if sym.Type.BlockName != tt.expectBlockName {
					t.Errorf("BlockName = %q, want %q", sym.Type.BlockName, tt.expectBlockName)
				}
				if sym.DefToken.Line != tt.expectLn {
					t.Errorf("DefToken.Line = %d, want %d", sym.DefToken.Line, tt.expectLn)
				}
				if sym.DefToken.Column != tt.expectCol {
					t.Errorf("DefToken.Column = %d, want %d", sym.DefToken.Column, tt.expectCol)
				}
			}
		})
	}
}

// TestSymbolTableBlockName verifies that BlockName on the stored Type is preserved.
func TestSymbolTableBlockName(t *testing.T) {
	st := NewSymbolTable()
	st.Define("gpt4", NewBlockRefType("gpt4", nil), token.Token{})

	typ, ok := st.Lookup("gpt4")
	if !ok {
		t.Fatal("expected gpt4 to be defined")
	}
	if typ.BlockName != "gpt4" {
		t.Errorf("BlockName = %q, want %q", typ.BlockName, "gpt4")
	}
}

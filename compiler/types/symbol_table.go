package types

import "github.com/thakee/orca/compiler/token"

// Symbol holds the resolved type and definition location for a named block.
type Symbol struct {
	Type     Type        // the block reference type
	DefToken token.Token // the name token where this symbol was defined
}

// SymbolTable maps block names to their resolved types and definition
// locations. Built by walking all BlockStatements before analyzing
// assignments, so identifiers like "gpt4" can be resolved to BlockRef(model).
type SymbolTable struct {
	symbols map[string]Symbol
}

// NewSymbolTable creates an empty symbol table.
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{symbols: make(map[string]Symbol)}
}

// Define adds a symbol to the table with its definition token.
// Overwrites if already defined.
func (st *SymbolTable) Define(name string, typ Type, defToken token.Token) {
	st.symbols[name] = Symbol{Type: typ, DefToken: defToken}
}

// Lookup returns the type for a symbol and whether it was found.
func (st *SymbolTable) Lookup(name string) (Type, bool) {
	sym, ok := st.symbols[name]
	return sym.Type, ok
}

// LookupSymbol returns the full symbol (type + definition location) and
// whether it was found.
func (st *SymbolTable) LookupSymbol(name string) (Symbol, bool) {
	sym, ok := st.symbols[name]
	return sym, ok
}

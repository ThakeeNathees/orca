package types

// SymbolTable maps block names to their resolved types. Built by walking
// all BlockStatements before analyzing assignments, so identifiers like
// "gpt4" can be resolved to BlockRef(model).
type SymbolTable struct {
	symbols map[string]Type
}

// NewSymbolTable creates an empty symbol table.
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{symbols: make(map[string]Type)}
}

// Define adds a symbol to the table. Overwrites if already defined.
func (st *SymbolTable) Define(name string, typ Type) {
	st.symbols[name] = typ
}

// Lookup returns the type for a symbol and whether it was found.
func (st *SymbolTable) Lookup(name string) (Type, bool) {
	typ, ok := st.symbols[name]
	return typ, ok
}

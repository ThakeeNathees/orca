package types

import "github.com/thakee/orca/compiler/token"

// Symbol holds the resolved type and definition location for a named block.
type Symbol struct {
	Type     Type        // the block reference type
	DefToken token.Token // the name token where this symbol was defined
}

// Scope holds symbols defined at a single nesting level. Each scope has an
// optional parent pointer forming a chain from innermost (e.g. lambda params)
// to the root (top-level blocks).
type Scope struct {
	symbols map[string]Symbol
	parent  *Scope
}

// SymbolTable maps block names to their resolved types and definition
// locations. Built by walking all BlockStatements before analyzing
// assignments, so identifiers like "gpt4" can be resolved to BlockRef(model).
// Supports nested scopes via PushScope/PopScope for lambda parameters.
type SymbolTable struct {
	current *Scope
}

// NewSymbolTable creates an empty symbol table with a root scope.
func NewSymbolTable() SymbolTable {
	return SymbolTable{current: &Scope{symbols: make(map[string]Symbol)}}
}

// PushScope creates a new child scope and sets it as the current scope.
// Symbols defined after this call are local to the new scope.
func (st *SymbolTable) PushScope() {
	st.current = &Scope{symbols: make(map[string]Symbol), parent: st.current}
}

// PopScope restores the parent scope as current, discarding the child scope.
// Panics if called on the root scope.
func (st *SymbolTable) PopScope() {
	if st.current.parent == nil {
		panic("BUG: PopScope called on root scope")
	}
	st.current = st.current.parent
}

// Define adds a symbol to the current scope with its definition token.
// Overwrites if already defined in the current scope.
func (st *SymbolTable) Define(name string, typ Type, defToken token.Token) {
	st.current.symbols[name] = Symbol{Type: typ, DefToken: defToken}
}

// Lookup returns the type for a symbol, walking up the scope chain.
func (st *SymbolTable) Lookup(name string) (Type, bool) {
	for scope := st.current; scope != nil; scope = scope.parent {
		if sym, ok := scope.symbols[name]; ok {
			return sym.Type, true
		}
	}
	return Type{}, false
}

// LookupSymbol returns the full symbol (type + definition location),
// walking up the scope chain.
func (st *SymbolTable) LookupSymbol(name string) (Symbol, bool) {
	for scope := st.current; scope != nil; scope = scope.parent {
		if sym, ok := scope.symbols[name]; ok {
			return sym, true
		}
	}
	return Symbol{}, false
}

// GetSymbols returns a merged view of all symbols visible from the current scope.
// Child scope symbols override parent symbols with the same name.
func (st *SymbolTable) GetSymbols() map[string]Symbol {
	merged := make(map[string]Symbol)
	// Collect scopes from root to current so child overrides parent.
	var chain []*Scope
	for scope := st.current; scope != nil; scope = scope.parent {
		chain = append(chain, scope)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		for name, sym := range chain[i].symbols {
			merged[name] = sym
		}
	}
	return merged
}

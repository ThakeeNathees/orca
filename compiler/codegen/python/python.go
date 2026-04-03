// Package python provides shared utilities for all Python-targeting codegen backends.
package python

import (
	"fmt"
	"strings"
)

// ImportSymbol names one symbol in a `from module import ...` statement.
// Name is the symbol as exposed by the module. Alias is optional; when non-empty,
// the emitted form is `Name as Alias`.
type ImportSymbol struct {
	Name  string
	Alias string
}

// PythonImport describes a single Python import for codegen and dependency tracking.
//
// Two forms are supported:
//   - From-import: FromImport true and Symbols non-empty → `from Module import ...`
//   - Module import: FromImport false → `import Module` or `import Module as ModuleAlias`
//
// Module is the dotted module path (e.g. "typing", "langchain_openai").
// Package is the PyPI distribution name required to install that module; use "" for
// the standard library or other environments where no pip package applies.
type PythonImport struct {
	Package     string
	Module      string
	ModuleAlias string
	FromImport  bool
	Symbols     []ImportSymbol
}

// Source returns one Python import line (no trailing newline). Invalid combinations
// (e.g. from-import with no symbols) return an empty string.
func (p PythonImport) Source() string {
	if p.FromImport {
		if p.ModuleAlias != "" {
			return ""
		}
		if p.Module == "" || len(p.Symbols) == 0 {
			return ""
		}
		var parts []string
		for _, sym := range p.Symbols {
			if sym.Name == "" {
				return ""
			}
			if sym.Alias != "" {
				parts = append(parts, sym.Name+" as "+sym.Alias)
			} else {
				parts = append(parts, sym.Name)
			}
		}
		return fmt.Sprintf("from %s import %s", p.Module, strings.Join(parts, ", "))
	}
	if len(p.Symbols) > 0 {
		return ""
	}
	if p.Module == "" {
		return ""
	}
	if p.ModuleAlias != "" {
		return fmt.Sprintf("import %s as %s", p.Module, p.ModuleAlias)
	}
	return "import " + p.Module
}

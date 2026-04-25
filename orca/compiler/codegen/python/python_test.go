package python

import (
	"testing"
)

// TestPythonImportSource verifies PythonImport.Source for from-import and module-import forms.
func TestPythonImportSource(t *testing.T) {
	tests := []struct {
		name     string
		imp      PythonImport
		expected string
	}{
		{
			name: "from-import single symbol stdlib",
			imp: PythonImport{
				Module:     "typing",
				Package:    "",
				FromImport: true,
				Symbols:    []ImportSymbol{{Name: "TypedDict"}},
			},
			expected: "from typing import TypedDict",
		},
		{
			name: "from-import alias",
			imp: PythonImport{
				Module:     "collections",
				Package:    "",
				FromImport: true,
				Symbols:    []ImportSymbol{{Name: "OrderedDict", Alias: "OD"}},
			},
			expected: "from collections import OrderedDict as OD",
		},
		{
			name: "from-import multiple symbols",
			imp: PythonImport{
				Module:     "typing",
				Package:    "",
				FromImport: true,
				Symbols: []ImportSymbol{
					{Name: "Any"},
					{Name: "Optional", Alias: "Opt"},
				},
			},
			expected: "from typing import Any, Optional as Opt",
		},
		{
			name: "from-import with pip package metadata only",
			imp: PythonImport{
				Module:     "langchain_openai",
				Package:    "langchain-openai",
				FromImport: true,
				Symbols:    []ImportSymbol{{Name: "ChatOpenAI"}},
			},
			expected: "from langchain_openai import ChatOpenAI",
		},
		{
			name: "import module",
			imp: PythonImport{
				Module:  "sys",
				Package: "",
			},
			expected: "import sys",
		},
		{
			name: "import module as alias",
			imp: PythonImport{
				Module:      "numpy",
				Package:     "numpy",
				ModuleAlias: "np",
			},
			expected: "import numpy as np",
		},
		{
			name: "invalid empty from-import symbols",
			imp: PythonImport{
				Module:     "typing",
				FromImport: true,
				Symbols:    nil,
			},
			expected: "",
		},
		{
			name: "invalid from-import with module alias",
			imp: PythonImport{
				Module:      "typing",
				FromImport:  true,
				ModuleAlias: "t",
				Symbols:     []ImportSymbol{{Name: "Any"}},
			},
			expected: "",
		},
		{
			name: "invalid module-import with symbols set",
			imp: PythonImport{
				Module:  "sys",
				Symbols: []ImportSymbol{{Name: "path"}},
			},
			expected: "",
		},
		{
			name:     "invalid empty module name",
			imp:      PythonImport{},
			expected: "",
		},
		{
			name: "invalid empty symbol name in from-import",
			imp: PythonImport{
				Module:     "typing",
				FromImport: true,
				Symbols:    []ImportSymbol{{Name: ""}},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.imp.Source()
			if got != tt.expected {
				t.Errorf("Source(): expected %q, got %q", tt.expected, got)
			}
		})
	}
}

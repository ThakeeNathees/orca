package python

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

// TestExprToPython verifies conversion of AST expressions to Python source.
func TestExprToPython(t *testing.T) {
	tests := []struct {
		name     string
		expr     ast.Expression
		expected string
	}{
		{
			name:     "string literal",
			expr:     &ast.StringLiteral{Value: "hello"},
			expected: `"hello"`,
		},
		{
			name:     "string with quotes",
			expr:     &ast.StringLiteral{Value: `say "hi"`},
			expected: `"say \"hi\""`,
		},
		{
			name:     "raw string with lang",
			expr:     &ast.StringLiteral{Value: "hello world", Lang: "md"},
			expected: `"hello world"`,
		},
		{
			name:     "raw string multiline",
			expr:     &ast.StringLiteral{Value: "line one\nline two", Lang: "py"},
			expected: "\"line one\\nline two\"",
		},
		{
			name:     "integer literal",
			expr:     &ast.IntegerLiteral{Value: 42},
			expected: "42",
		},
		{
			name:     "zero integer",
			expr:     &ast.IntegerLiteral{Value: 0},
			expected: "0",
		},
		{
			name:     "negative integer",
			expr:     &ast.IntegerLiteral{Value: -1},
			expected: "-1",
		},
		{
			name:     "float literal",
			expr:     &ast.FloatLiteral{Value: 3.14},
			expected: "3.14",
		},
		{
			name:     "float whole number",
			expr:     &ast.FloatLiteral{Value: 1.0},
			expected: "1.0",
		},
		{
			name:     "float zero",
			expr:     &ast.FloatLiteral{Value: 0.0},
			expected: "0.0",
		},
		{
			name:     "boolean true",
			expr:     &ast.BooleanLiteral{Value: true},
			expected: "True",
		},
		{
			name:     "boolean false",
			expr:     &ast.BooleanLiteral{Value: false},
			expected: "False",
		},
		{
			name:     "null literal",
			expr:     &ast.NullLiteral{},
			expected: "None",
		},
		{
			name:     "identifier",
			expr:     &ast.Identifier{Value: "my_var"},
			expected: "my_var",
		},
		{
			name: "member access",
			expr: &ast.MemberAccess{
				Object: &ast.Identifier{Value: "config"},
				Member: "timeout",
			},
			expected: "config.timeout",
		},
		{
			name: "nested member access",
			expr: &ast.MemberAccess{
				Object: &ast.MemberAccess{
					Object: &ast.Identifier{Value: "a"},
					Member: "b",
				},
				Member: "c",
			},
			expected: "a.b.c",
		},
		{
			name: "subscription with integer",
			expr: &ast.Subscription{
				Object: &ast.Identifier{Value: "items"},
				Index:  &ast.IntegerLiteral{Value: 0},
			},
			expected: "items[0]",
		},
		{
			name: "subscription with string key",
			expr: &ast.Subscription{
				Object: &ast.Identifier{Value: "data"},
				Index:  &ast.StringLiteral{Value: "key"},
			},
			expected: `data["key"]`,
		},
		{
			name:     "empty list",
			expr:     &ast.ListLiteral{Elements: []ast.Expression{}},
			expected: "[]",
		},
		{
			name: "list with elements",
			expr: &ast.ListLiteral{
				Elements: []ast.Expression{
					&ast.IntegerLiteral{Value: 1},
					&ast.IntegerLiteral{Value: 2},
					&ast.IntegerLiteral{Value: 3},
				},
			},
			expected: "[1, 2, 3]",
		},
		{
			name: "list with mixed types",
			expr: &ast.ListLiteral{
				Elements: []ast.Expression{
					&ast.StringLiteral{Value: "a"},
					&ast.IntegerLiteral{Value: 1},
					&ast.BooleanLiteral{Value: true},
				},
			},
			expected: `["a", 1, True]`,
		},
		{
			name:     "empty map",
			expr:     &ast.MapLiteral{Entries: []ast.MapEntry{}},
			expected: "{}",
		},
		{
			name: "map with entries",
			expr: &ast.MapLiteral{
				Entries: []ast.MapEntry{
					{Key: &ast.StringLiteral{Value: "name"}, Value: &ast.StringLiteral{Value: "orca"}},
					{Key: &ast.StringLiteral{Value: "version"}, Value: &ast.IntegerLiteral{Value: 1}},
				},
			},
			expected: `{"name": "orca", "version": 1}`,
		},
		{
			name: "binary expression",
			expr: &ast.BinaryExpression{
				Left:     &ast.IntegerLiteral{Value: 1},
				Operator: token.Token{Literal: "+"},
				Right:    &ast.IntegerLiteral{Value: 2},
			},
			expected: "1 + 2",
		},
		{
			name: "binary expression with strings",
			expr: &ast.BinaryExpression{
				Left:     &ast.StringLiteral{Value: "hello"},
				Operator: token.Token{Literal: "+"},
				Right:    &ast.StringLiteral{Value: " world"},
			},
			expected: `"hello" + " world"`,
		},
		{
			name: "call expression no args",
			expr: &ast.CallExpression{
				Callee:    &ast.Identifier{Value: "foo"},
				Arguments: []ast.Expression{},
			},
			expected: "foo()",
		},
		{
			name: "call expression with args",
			expr: &ast.CallExpression{
				Callee: &ast.Identifier{Value: "max"},
				Arguments: []ast.Expression{
					&ast.IntegerLiteral{Value: 1},
					&ast.IntegerLiteral{Value: 2},
				},
			},
			expected: "max(1, 2)",
		},
		{
			name: "method call via member access",
			expr: &ast.CallExpression{
				Callee: &ast.MemberAccess{
					Object: &ast.Identifier{Value: "obj"},
					Member: "method",
				},
				Arguments: []ast.Expression{
					&ast.StringLiteral{Value: "arg"},
				},
			},
			expected: `obj.method("arg")`,
		},
		{
			name: "nested list in map",
			expr: &ast.MapLiteral{
				Entries: []ast.MapEntry{
					{
						Key: &ast.StringLiteral{Value: "items"},
						Value: &ast.ListLiteral{
							Elements: []ast.Expression{
								&ast.IntegerLiteral{Value: 1},
								&ast.IntegerLiteral{Value: 2},
							},
						},
					},
				},
			},
			expected: `{"items": [1, 2]}`,
		},
		{
			name:     "unknown expression type returns None",
			expr:     &ast.BlockExpression{},
			expected: "None",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OrcaToPythonExpression(tt.expr)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// TestFormatFloat verifies float formatting for Python output.
func TestFormatFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{"zero", 0.0, "0.0"},
		{"whole number", 1.0, "1.0"},
		{"decimal", 3.14, "3.14"},
		{"small decimal", 0.001, "0.001"},
		{"large number", 1000000.0, "1e+06"},
		{"negative", -2.5, "-2.5"},
		{"negative whole", -1.0, "-1.0"},
		{"half", 0.5, "0.5"},
		{"very small", 0.0001, "0.0001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatFloat(tt.input)
			if got != tt.expected {
				t.Errorf("FormatFloat(%v): expected %q, got %q", tt.input, tt.expected, got)
			}
		})
	}
}

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

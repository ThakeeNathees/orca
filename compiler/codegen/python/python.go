// Package python provides shared utilities for all Python-targeting codegen backends.
package python

import (
	"fmt"
	"strings"

	"github.com/thakee/orca/compiler/ast"
)

// OrcaToPythonExpression converts an AST expression to its Python representation.
// Shared across all Python-targeting backends.
func OrcaToPythonExpression(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.StringLiteral:
		return fmt.Sprintf("%q", e.Value)
	case *ast.IntegerLiteral:
		return fmt.Sprintf("%d", e.Value)
	case *ast.FloatLiteral:
		return FormatFloat(e.Value)
	case *ast.BooleanLiteral:
		if e.Value {
			return "True"
		}
		return "False"
	case *ast.NullLiteral:
		return "None"
	case *ast.Identifier:
		return e.Value
	case *ast.MemberAccess:
		return OrcaToPythonExpression(e.Object) + "." + e.Member
	case *ast.Subscription:
		return OrcaToPythonExpression(e.Object) + "[" + OrcaToPythonExpression(e.Index) + "]"
	case *ast.ListLiteral:
		var elems []string
		for _, el := range e.Elements {
			elems = append(elems, OrcaToPythonExpression(el))
		}
		return "[" + strings.Join(elems, ", ") + "]"
	case *ast.MapLiteral:
		var entries []string
		for _, entry := range e.Entries {
			entries = append(entries, OrcaToPythonExpression(entry.Key)+": "+OrcaToPythonExpression(entry.Value))
		}
		return "{" + strings.Join(entries, ", ") + "}"
	case *ast.BinaryExpression:
		return OrcaToPythonExpression(e.Left) + " " + e.Operator.Literal + " " + OrcaToPythonExpression(e.Right)
	case *ast.CallExpression:
		var args []string
		for _, arg := range e.Arguments {
			args = append(args, OrcaToPythonExpression(arg))
		}
		return OrcaToPythonExpression(e.Callee) + "(" + strings.Join(args, ", ") + ")"
	default:
		return "None"
	}
}

// FormatFloat formats a float without unnecessary trailing zeros,
// ensuring a decimal point for valid Python syntax.
func FormatFloat(f float64) string {
	s := fmt.Sprintf("%g", f)
	if !strings.Contains(s, ".") && !strings.Contains(s, "e") {
		s += ".0"
	}
	return s
}

// Package codegen defines the code generation interface and shared utilities.
// Backend implementations (e.g., langgraph) live in sub-packages.
package codegen

import (
	"fmt"
	"strings"

	"github.com/thakee/orca/compiler/ast"
)

// Output holds the generated files from a codegen backend.
type Output struct {
	MainPy        string // generated Python source
	PyProjectTOML string // generated pyproject.toml
}

// Backend defines a code generation target. Each backend (LangGraph, CrewAI, etc.)
// implements this interface to produce Python code from the IR.
type Backend interface {
	Generate() Output
}

// ExprToPython converts an AST expression to its Python representation.
// Shared across all Python-targeting backends.
func ExprToPython(expr ast.Expression) string {
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
		return ExprToPython(e.Object) + "." + e.Member
	case *ast.Subscription:
		return ExprToPython(e.Object) + "[" + ExprToPython(e.Index) + "]"
	case *ast.ListLiteral:
		var elems []string
		for _, el := range e.Elements {
			elems = append(elems, ExprToPython(el))
		}
		return "[" + strings.Join(elems, ", ") + "]"
	case *ast.MapLiteral:
		var entries []string
		for _, entry := range e.Entries {
			entries = append(entries, ExprToPython(entry.Key)+": "+ExprToPython(entry.Value))
		}
		return "{" + strings.Join(entries, ", ") + "}"
	case *ast.BinaryExpression:
		return ExprToPython(e.Left) + " " + e.Operator.Literal + " " + ExprToPython(e.Right)
	case *ast.CallExpression:
		var args []string
		for _, arg := range e.Arguments {
			args = append(args, ExprToPython(arg))
		}
		return ExprToPython(e.Callee) + "(" + strings.Join(args, ", ") + ")"
	default:
		return "None"
	}
}

// SourceComment formats a source mapping comment like "agents.oc:42" or "line 42".
func SourceComment(file string, line int) string {
	if file != "" {
		return fmt.Sprintf("%s:%d", file, line)
	}
	return fmt.Sprintf("line %d", line)
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

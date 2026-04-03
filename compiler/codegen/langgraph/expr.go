package langgraph

import (
	"fmt"
	"strings"

	"github.com/thakee/orca/compiler/ast"
)

// exprToSource converts an AST expression to its Python source representation.
func exprToSource(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.StringLiteral:
		return fmt.Sprintf("%q", e.Value)
	case *ast.IntegerLiteral:
		return fmt.Sprintf("%d", e.Value)
	case *ast.FloatLiteral:
		return formatFloat(e.Value)
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
		return exprToSource(e.Object) + "." + e.Member
	case *ast.Subscription:
		return exprToSource(e.Object) + "[" + exprToSource(e.Index) + "]"
	case *ast.ListLiteral:
		var elems []string
		for _, el := range e.Elements {
			elems = append(elems, exprToSource(el))
		}
		return "[" + strings.Join(elems, ", ") + "]"
	case *ast.MapLiteral:
		var entries []string
		for _, entry := range e.Entries {
			entries = append(entries, exprToSource(entry.Key)+": "+exprToSource(entry.Value))
		}
		return "{" + strings.Join(entries, ", ") + "}"
	case *ast.BinaryExpression:
		return exprToSource(e.Left) + " " + e.Operator.Literal + " " + exprToSource(e.Right)
	case *ast.CallExpression:
		var args []string
		for _, arg := range e.Arguments {
			args = append(args, exprToSource(arg))
		}
		return exprToSource(e.Callee) + "(" + strings.Join(args, ", ") + ")"
	case *ast.BlockExpression:
		return blockCallSource(&e.BlockBody, "")
	default:
		return "None"
	}
}

// blockCallSource generates a Python expression calling the orca runtime for
// a given block body: orca.<kind>(key=val, ...).
//
// When indent is non-empty, each keyword argument is placed on its own line
// prefixed by that indent string, producing multi-line output:
//
//	orca.model(
//	    provider="openai",
//	    model_name="gpt-4o",
//	)
//
// When indent is empty, everything is on one line (used for inline blocks).
func blockCallSource(body *ast.BlockBody, indent string) string {
	var sb strings.Builder
	sb.WriteString("orca.")
	sb.WriteString(body.Kind.String())
	sb.WriteString("(")

	if indent == "" {
		for _, assign := range body.Assignments {
			sb.WriteString(assign.Name)
			sb.WriteString("=")
			sb.WriteString(exprToSource(assign.Value))
			sb.WriteString(", ")
		}
	} else {
		for _, assign := range body.Assignments {
			sb.WriteString("\n")
			sb.WriteString(indent)
			sb.WriteString(assign.Name)
			sb.WriteString("=")
			sb.WriteString(exprToSource(assign.Value))
			sb.WriteString(",")
		}
		if len(body.Assignments) > 0 {
			sb.WriteString("\n")
		}
	}

	sb.WriteString(")")
	return sb.String()
}

// formatFloat formats a float without unnecessary trailing zeros,
// ensuring a decimal point for valid Python syntax.
func formatFloat(f float64) string {
	s := fmt.Sprintf("%g", f)
	if !strings.Contains(s, ".") && !strings.Contains(s, "e") {
		s += ".0"
	}
	return s
}

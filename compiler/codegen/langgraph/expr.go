package langgraph

import (
	"fmt"
	"math"
	"strings"

	"github.com/thakee/orca/compiler/ast"
)

// exprToSource converts an AST expression to its Python source representation.
//
// It must handle every concrete type that implements ast.Expression (see ast.go).
// Unknown or nil interfaces are handled explicitly: nil becomes Python None;
// any other type panics so mistakes surface immediately instead of emitting
// invalid Python.
func exprToSource(expr ast.Expression) string {
	switch e := expr.(type) {
	case nil:
		return "None"
	case *ast.StringLiteral:
		return fmt.Sprintf("%q", e.Value)
	case *ast.NumberLiteral:
		return formatFloat(e.Value)
	case *ast.Identifier:
		switch e.Value {
		case "null":
			return "None"
		case "true":
			return "True"
		case "false":
			return "False"
		case "string":
			return "str"
		case "number":
			return "float"
		default:
			return e.Value
		}
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
	case *ast.TernaryExpression:
		return "(" + exprToSource(e.TrueExpr) + " if " + exprToSource(e.Condition) + " else " + exprToSource(e.FalseExpr) + ")"
	case *ast.BlockExpression:
		return blockCallSource(&e.BlockBody, "")
	default:
		panic(fmt.Sprintf("langgraph.exprToSource: unsupported ast.Expression type %T", e))
	}
}

// annotationToSource emits one `__orca_meta("name", ...args)` from an AST annotation.
func annotationToSource(ann *ast.Annotation) string {
	var sb strings.Builder
	sb.WriteString(orcaPrefix + "meta(")
	sb.WriteString(fmt.Sprintf("%q", ann.Name))
	for _, arg := range ann.Arguments {
		sb.WriteString(", ")
		sb.WriteString(exprToSource(arg))
	}
	sb.WriteString(")")
	return sb.String()
}

// indentNonEmptyLines prefixes each non-empty line of s with prefix (for nesting
// already-indented Python fragments inside a larger multi-line construct).
func indentNonEmptyLines(s, prefix string) string {
	if s == "" {
		return prefix + s
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		if lines[i] != "" {
			lines[i] = prefix + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

// annotationsListSourceMultiline builds a multi-line Python list of orca.meta(...)
// calls. argIndent is the indentation of the opening "[" and closing "]" lines;
// list elements are indented one level deeper.
func annotationsListSourceMultiline(anns []*ast.Annotation, argIndent string) string {
	if len(anns) == 0 {
		return "[]"
	}
	metaIndent := argIndent + "    "
	var b strings.Builder
	b.WriteString(argIndent)
	b.WriteString("[\n")
	for _, a := range anns {
		b.WriteString(metaIndent)
		b.WriteString(annotationToSource(a))
		b.WriteString(",\n")
	}
	b.WriteString(argIndent)
	b.WriteString("]")
	return b.String()
}

// wrapWithMetaIfNeeded returns inner unchanged when there are no annotations.
// Otherwise emits a multi-line orca.with_meta(...) so nested blocks and long
// meta lists stay readable. argIndent prefixes each line of the inner value and
// the meta list contents; closeIndent prefixes the closing ")" ("" at top level,
// fieldIndent when the with_meta is a field RHS so ")" aligns with "key=").
func wrapWithMetaIfNeeded(inner string, anns []*ast.Annotation, argIndent, closeIndent string) string {
	if len(anns) == 0 {
		return inner
	}
	listSrc := annotationsListSourceMultiline(anns, argIndent)
	innerIndented := indentNonEmptyLines(inner, argIndent)
	var sb strings.Builder
	sb.WriteString(orcaPrefix + "with_meta(\n")
	sb.WriteString(innerIndented)
	sb.WriteString(",\n")
	sb.WriteString(listSrc)
	sb.WriteString(",\n")
	sb.WriteString(closeIndent)
	sb.WriteString(")")
	return sb.String()
}

// assignmentValueSource emits the RHS for a block field, wrapping with with_meta
// only when the assignment carries annotations. fieldIndent is the block body's
// line prefix for assignments ("    " in multi-line blocks, "" when inline).
func assignmentValueSource(assign *ast.Assignment, fieldIndent string) string {
	if assign == nil {
		return "None"
	}
	val := exprToSource(assign.Value)
	argIndent := fieldIndent + "    "
	return wrapWithMetaIfNeeded(val, assign.Annotations, argIndent, fieldIndent)
}

// topLevelBlockSource emits the full Python RHS for a top-level block statement,
// including optional block-level annotations around the orca.<kind>(...) call.
func topLevelBlockSource(block *ast.BlockStatement) string {
	if block == nil {
		panic("BUG: topLevelBlockSource called with nil block")
	}
	inner := blockCallSource(&block.BlockBody, "    ")
	return wrapWithMetaIfNeeded(inner, block.Annotations, "    ", "")
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
	sb.WriteString(fmt.Sprintf("%sblock(%q, ", orcaPrefix, body.Kind))

	if indent == "" {
		for _, assign := range body.Assignments {
			sb.WriteString(assign.Name)
			sb.WriteString("=")
			sb.WriteString(assignmentValueSource(assign, indent))
			sb.WriteString(", ")
		}
	} else {
		for _, assign := range body.Assignments {
			sb.WriteString("\n")
			sb.WriteString(indent)
			sb.WriteString(assign.Name)
			sb.WriteString("=")
			sb.WriteString(assignmentValueSource(assign, indent))
			sb.WriteString(",")
		}
		if len(body.Assignments) > 0 {
			sb.WriteString("\n")
		}
	}

	sb.WriteString(")")
	return sb.String()
}

// formatFloat formats numbers for Python source. Whole values use integer
// literals (no ".0"); fractional values use %g.
func formatFloat(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return fmt.Sprintf("%g", f)
	}
	if f == math.Trunc(f) {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%g", f)
}

package langgraph

import (
	"fmt"
	"math"
	"strings"

	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
)

// exprToSource converts an AST expression to its Python source representation.
//
// It must handle every concrete type that implements ast.Expression (see ast.go).
// Unknown or nil interfaces are handled explicitly: nil becomes Python None;
// any other type panics so mistakes surface immediately instead of emitting
// invalid Python.
func (b *LangGraphBackend) exprToSource(expr ast.Expression) string {

	// TODO: We're not folding block expressions cause currently we are depend on the block
	// body (the defined block one) to generate workflow nodes, but that behavior should be changed.
	//
	// Here is another !! BUG !! if we use inline block.
	//     "a": _orca__block("agent", model=_orca__block("model", provider="openai", model_name="gpt-4o", ), ...)
	//     "b": _orca__block("agent", model=_orca__block("model", model_name="gpt-4o", provider="openai", ), ...)
	// Source order is provider then model_name in both cases. constValToSource's ConstMap and ConstBlock cases iterate constVal.KeyValue which is a
	// map[string]ConstValue — Go map iteration is randomized, so every codegen run produces different Python. Goldens will flake.
	constVal, ok := b.Program.ConstFoldCache[expr]
	if ok && constVal.Kind != analyzer.ConstUnknown && constVal.Kind != analyzer.ConstBlock && constVal.Kind != analyzer.ConstLambda {
		return constValToSource(b, constVal)
	}

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
		return b.exprToSource(e.Object) + "." + e.Member
	case *ast.Subscription:
		var indices []string
		for _, idx := range e.Indices {
			indices = append(indices, b.exprToSource(idx))
		}
		return b.exprToSource(e.Object) + "[" + strings.Join(indices, ", ") + "]"
	case *ast.ListLiteral:
		var elems []string
		for _, el := range e.Elements {
			elems = append(elems, b.exprToSource(el))
		}
		return "[" + strings.Join(elems, ", ") + "]"
	case *ast.MapLiteral:
		var entries []string
		for _, entry := range e.Entries {
			entries = append(entries, b.exprToSource(entry.Key)+": "+b.exprToSource(entry.Value))
		}
		return "{" + strings.Join(entries, ", ") + "}"
	case *ast.BinaryExpression:
		if e.Operator.Type == token.ARROW {
			// FIXME: Dont hard code like this
			// "_orca__block("workflow_chain", left, right)"
			return orcaPrefix + "block(\"" + types.AnnotationWorkflowChain +
				"\", left=" + b.exprToSource(e.Left) +
				", right=" + b.exprToSource(e.Right) + ")"
		}
		return b.exprToSource(e.Left) + " " + e.Operator.Literal + " " + b.exprToSource(e.Right)
	case *ast.CallExpression:
		var args []string
		for _, arg := range e.Arguments {
			args = append(args, b.exprToSource(arg))
		}
		return b.exprToSource(e.Callee) + "(" + strings.Join(args, ", ") + ")"
	case *ast.TernaryExpression:
		return "(" + b.exprToSource(e.TrueExpr) + " if " + b.exprToSource(e.Condition) + " else " + b.exprToSource(e.FalseExpr) + ")"
	case *ast.Lambda:
		var params []string
		for _, p := range e.Params {
			params = append(params, p.Name.Value)
		}
		if len(params) == 0 {
			return "(lambda: " + b.exprToSource(e.Body) + ")"
		}
		return "(lambda " + strings.Join(params, ", ") + ": " + b.exprToSource(e.Body) + ")"
	case *ast.BlockExpression:
		return blockCallSource(b, &e.BlockBody, "")
	default:
		panic(fmt.Sprintf("langgraph.exprToSource: unsupported ast.Expression type %T", e))
	}
}

// annotationToSource emits one `__orca_meta("name", ...args)` from an AST annotation.
func annotationToSource(b *LangGraphBackend, ann *ast.Annotation) string {
	var sb strings.Builder
	sb.WriteString(orcaPrefix + "meta(")
	sb.WriteString(fmt.Sprintf("%q", ann.Name))
	for _, arg := range ann.Arguments {
		sb.WriteString(", ")
		sb.WriteString(b.exprToSource(arg))
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
func annotationsListSourceMultiline(b *LangGraphBackend, anns []*ast.Annotation, argIndent string) string {
	if len(anns) == 0 {
		return "[]"
	}
	metaIndent := argIndent + "    "
	var sb strings.Builder
	sb.WriteString(argIndent)
	sb.WriteString("[\n")
	for _, a := range anns {
		sb.WriteString(metaIndent)
		sb.WriteString(annotationToSource(b, a))
		sb.WriteString(",\n")
	}
	sb.WriteString(argIndent)
	sb.WriteString("]")
	return sb.String()
}

// wrapWithMetaIfNeeded returns inner unchanged when there are no annotations.
// Otherwise emits a multi-line orca.with_meta(...) so nested blocks and long
// meta lists stay readable. argIndent prefixes each line of the inner value and
// the meta list contents; closeIndent prefixes the closing ")" ("" at top level,
// fieldIndent when the with_meta is a field RHS so ")" aligns with "key=").
func wrapWithMetaIfNeeded(b *LangGraphBackend, inner string, anns []*ast.Annotation, argIndent, closeIndent string) string {
	if len(anns) == 0 {
		return inner
	}
	listSrc := annotationsListSourceMultiline(b, anns, argIndent)
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
func assignmentValueSource(b *LangGraphBackend, assign *ast.Assignment, fieldIndent string) string {
	if assign == nil {
		return "None"
	}
	val := b.exprToSource(assign.Value)
	argIndent := fieldIndent + "    "
	return wrapWithMetaIfNeeded(b, val, assign.Annotations, argIndent, fieldIndent)
}

// topLevelBlockSource emits the full Python RHS for a top-level block statement,
// including optional block-level annotations around the orca.<kind>(...) call.
func topLevelBlockSource(b *LangGraphBackend, block *ast.BlockStatement) string {
	if block == nil {
		panic("BUG: topLevelBlockSource called with nil block")
	}
	inner := blockCallSource(b, &block.BlockBody, "    ")
	return wrapWithMetaIfNeeded(b, inner, block.Annotations, "    ", "")
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
func blockCallSource(b *LangGraphBackend, body *ast.BlockBody, indent string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%sblock(%q, ", orcaPrefix, body.Kind))

	if indent == "" {
		for _, assign := range body.Assignments {
			sb.WriteString(assign.Name)
			sb.WriteString("=")
			sb.WriteString(assignmentValueSource(b, assign, indent))
			sb.WriteString(", ")
		}
	} else {
		for _, assign := range body.Assignments {
			sb.WriteString("\n")
			sb.WriteString(indent)
			sb.WriteString(assign.Name)
			sb.WriteString("=")
			sb.WriteString(assignmentValueSource(b, assign, indent))
			sb.WriteString(",")
		}
		if len(body.Assignments) > 0 {
			sb.WriteString("\n")
		}
	}

	sb.WriteString(")")
	return sb.String()
}

func constValToSource(b *LangGraphBackend, constVal analyzer.ConstValue) string {

	// TODO:
	// Cost folded blocks are not inlined, Rethink this situaion.
	// Also a lambda can return a block that has const value (compile time known) how to handle that?
	if constVal.Kind == analyzer.ConstUnknown ||
		constVal.Kind == analyzer.ConstBlock ||
		constVal.Kind == analyzer.ConstLambda {
		return b.exprToSource(constVal.Expr)
	}

	switch constVal.Kind {
	case analyzer.ConstString:
		return fmt.Sprintf("%q", constVal.Str)
	case analyzer.ConstNumber:
		return formatFloat(constVal.Number)
	case analyzer.ConstBool:
		if constVal.Bool {
			return "True"
		}
		return "False"
	case analyzer.ConstNull:
		return "None"
	case analyzer.ConstList:
		var elems []string
		for _, elem := range constVal.List {
			elems = append(elems, constValToSource(b, elem))
		}
		return "[" + strings.Join(elems, ", ") + "]"
	case analyzer.ConstMap:
		var entries []string
		for i, key := range constVal.Keys {
			// TODO: Handle escaped quotes and newlines in the string.
			value := constVal.Values[i]
			entries = append(entries, "\""+key+"\": "+constValToSource(b, value))
		}
		return "{" + strings.Join(entries, ", ") + "}"
	case analyzer.ConstBlock:
		// TODO: If the const block has k=v, expressions it set to Partial constant and
		// those expressions are not folded, we should handle them somehow.

		// blockCallSource and this uses almost same logic make sure they are in sync.
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%sblock(%q, ", orcaPrefix, constVal.BlockKind))
		for i, key := range constVal.Keys {
			value := constVal.Values[i]
			sb.WriteString(key)
			sb.WriteString("=")
			sb.WriteString(constValToSource(b, value))
			sb.WriteString(", ")
		}
		sb.WriteString(")")
		return sb.String()
	}

	panic(fmt.Sprintf("langgraph.constValToSource: unsupported analyzer.ConstKind %d", constVal.Kind))
}

// TODO: Move this to a helper function.
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

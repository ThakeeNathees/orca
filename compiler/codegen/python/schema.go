package python

import (
	"fmt"
	"strings"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/types"
)

// SchemaBlockToSource generates a Pydantic BaseModel class from a schema block.
// Uses the type system (ExprType in bootstrap mode) to resolve field types,
// producing proper Python type annotations.
//
//	schema research_report {
//	  @desc("The topic")
//	  topic = str
//	  score = float | null
//	}
//
// becomes:
//
//	class research_report(BaseModel):
//	    topic: str = Field(description="The topic")
//	    score: float | None = Field(default=None)
func SchemaBlockToSource(block *ast.BlockStatement) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "class %s(BaseModel):\n", block.Name)

	if len(block.Assignments) == 0 {
		sb.WriteString("    pass\n")
		return sb.String()
	}

	for _, assign := range block.Assignments {
		resolved := types.BlockSchemaTypeOfExpr(assign.Value, nil)
		isOptional := isOptional(resolved)
		desc := extractDesc(assign.Annotations)

		typeStr := orcaTypeToPythonTypeName(resolved)

		sb.WriteString("    ")
		sb.WriteString(assign.Name)
		sb.WriteString(": ")
		sb.WriteString(typeStr)

		// Build the Field() call or default value.
		if desc != "" && isOptional {
			fmt.Fprintf(&sb, " = Field(default=None, description=%q)", desc)
		} else if desc != "" {
			fmt.Fprintf(&sb, " = Field(description=%q)", desc)
		} else if isOptional {
			sb.WriteString(" = None")
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// OrcaTypeToPythonTypeName converts a resolved types.Type to a Python type annotation string.
// Exported for use by other codegen packages (e.g. workflow state generation).
func OrcaTypeToPythonTypeName(t types.Type) string {
	return orcaTypeToPythonTypeName(t)
}

func isOptional(t types.Type) bool {
	if t.Kind == types.Union {
		for _, m := range t.Members {
			if m.IsNull() {
				return true
			}
		}
	}
	return false
}

// orcaTypeToPythonTypeName converts a resolved types.Type to a Python type annotation string.
// Uses the type system's resolved types rather than raw AST pattern-matching,
// giving correct results for all type constructs (unions, generics, schema refs).
func orcaTypeToPythonTypeName(t types.Type) string {
	switch t.Kind {
	case types.BlockRef:
		return orcaSchemaToPythonTypeName(t)
	case types.List:
		if t.ElementType != nil {
			return "list[" + orcaTypeToPythonTypeName(*t.ElementType) + "]"
		}
		return "list"
	case types.Map:
		if t.ValueType != nil {
			return "dict[str, " + orcaTypeToPythonTypeName(*t.ValueType) + "]"
		}
		return "dict"
	case types.Union:
		parts := make([]string, len(t.Members))
		for i, m := range t.Members {
			parts[i] = orcaTypeToPythonTypeName(m)
		}
		return strings.Join(parts, " | ")
	default:
		return "Any"
	}
}

// orcaSchemaToPythonTypeName maps a BlockRef type to its Python annotation.
// Built-in primitives map directly; "any" → "Any", "null" → "None".
// User-defined schema names pass through as-is.
func orcaSchemaToPythonTypeName(t types.Type) string {
	if t.BlockName == "" {
		// Generic block reference (e.g. model, agent) — use "Any".
		return "Any"
	}
	switch t.BlockName {
	case "str", "int", "float", "bool":
		return t.BlockName
	case "any":
		return "Any"
	case "null":
		return "None"
	default:
		// User-defined schema name.
		return t.BlockName
	}
}

// extractDesc returns the description string from a @desc("...") annotation
// if present. Returns "" if no @desc annotation exists.
func extractDesc(anns []*ast.Annotation) string {
	for _, ann := range anns {
		if ann.Name == "desc" && len(ann.Arguments) == 1 {
			if str, ok := ann.Arguments[0].(*ast.StringLiteral); ok {
				return str.Value
			}
		}
	}
	return ""
}

// SchemaImport returns the pydantic import needed when schema blocks are present.
func SchemaImport() PythonImport {
	return PythonImport{
		Package:    "pydantic",
		Module:     "pydantic",
		FromImport: true,
		Symbols: []ImportSymbol{
			{Name: "BaseModel"},
			{Name: "Field"},
		},
	}
}

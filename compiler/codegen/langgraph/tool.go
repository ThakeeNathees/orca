package langgraph

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/codegen/python"
	"github.com/thakee/orca/compiler/token"
)

// defNameRe matches `def <name>` at the start of a Python function definition
// to extract the function name from an inline invoke raw string.
var defNameRe = regexp.MustCompile(`(?m)^\s*def\s+(\w+)`)

// resolvedToolInvoke holds the pre-processed invoke information for a single tool block.
type resolvedToolInvoke struct {
	invokeRef string               // Python identifier to use for invoke= in orca.tool(...)
	pyImport  *python.PythonImport // non-nil when invoke is a dotted import path
	verbatim  string               // non-empty when invoke is an inline raw string function def
}

// resolveToolInvokes pre-processes all tool blocks to determine how each invoke
// field should be generated. Collects import statements for dotted paths and
// validates inline raw string functions. Results are stored in resolvedTools
// keyed by block name.
func (b *LangGraphBackend) resolveToolInvokes() {
	tools := b.CollectBlocksByKind(token.BlockTool)
	if len(tools) == 0 {
		return
	}

	b.resolvedTools = make(map[string]resolvedToolInvoke, len(tools))

	for _, tool := range tools {
		invokeExpr, ok := tool.GetFieldExpression("invoke")
		if !ok {
			continue
		}

		str, isStr := invokeExpr.(*ast.StringLiteral)
		if !isStr {
			continue
		}

		if str.Lang == "py" {
			b.resolveInlineInvoke(tool, str)
		} else {
			b.resolveDottedPathInvoke(tool, str)
		}
	}
}

// resolveInlineInvoke processes an inline raw string invoke (```py ... ```).
// Extracts the function name via regex, warns if it doesn't match the block
// name, and prepares the verbatim function source with a renamed function.
func (b *LangGraphBackend) resolveInlineInvoke(tool *ast.BlockStatement, str *ast.StringLiteral) {
	renamed := tool.Name + "__invoke_verbatim"

	// Validation (missing def, name mismatch) is handled by the analyzer.
	matches := defNameRe.FindStringSubmatch(str.Value)
	if len(matches) < 2 {
		return
	}

	funcName := matches[1]

	// Rename the function in the source to avoid collision with the tool variable.
	source := strings.Replace(str.Value, "def "+funcName+"(", "def "+renamed+"(", 1)

	b.resolvedTools[tool.Name] = resolvedToolInvoke{
		invokeRef: renamed,
		verbatim:  source,
	}
}

// resolveDottedPathInvoke processes a dotted import path invoke (e.g. "package.module.callable").
// Parses the path into module and callable, and prepares the import statement.
func (b *LangGraphBackend) resolveDottedPathInvoke(tool *ast.BlockStatement, str *ast.StringLiteral) {
	path := str.Value
	lastDot := strings.LastIndex(path, ".")
	if lastDot < 0 {
		// No dot — treat as a bare callable name (no import needed).
		b.resolvedTools[tool.Name] = resolvedToolInvoke{
			invokeRef: path,
		}
		return
	}

	module := path[:lastDot]
	callable := path[lastDot+1:]

	imp := python.PythonImport{
		Module:     module,
		FromImport: true,
		Symbols:    []python.ImportSymbol{{Name: callable}},
	}

	b.resolvedTools[tool.Name] = resolvedToolInvoke{
		invokeRef: callable,
		pyImport:  &imp,
	}
}

// writeToolSection emits tool blocks with special handling for the invoke field.
// Inline raw string invokes emit the function definition verbatim before the
// tool variable; dotted path invokes reference the imported callable.
func (b *LangGraphBackend) writeToolSection(s *strings.Builder) {
	tools := b.CollectBlocksByKind(token.BlockTool)
	if len(tools) == 0 {
		return
	}

	s.WriteString("\n# --- Tools ---\n")

	for _, tool := range tools {
		resolved, ok := b.resolvedTools[tool.Name]
		if !ok {
			// No resolved invoke — emit the tool with generic block source.
			s.WriteString("\n")
			fmt.Fprintf(s, "%s = %s\n", tool.Name, topLevelBlockSource(tool))
			continue
		}

		// Emit verbatim function definition if present.
		if resolved.verbatim != "" {
			s.WriteString("\n")
			s.WriteString(resolved.verbatim)
			s.WriteString("\n")
		}

		// Emit orca.tool(...) with invoke as a callable reference.
		s.WriteString("\n")
		fmt.Fprintf(s, "%s = %s\n", tool.Name, toolBlockSource(tool, resolved.invokeRef))
	}
}

// toolBlockSource generates the orca.tool(...) call, substituting the invoke
// field value with the resolved callable reference instead of the original string.
func toolBlockSource(tool *ast.BlockStatement, invokeRef string) string {
	var sb strings.Builder
	sb.WriteString("orca.tool(")

	indent := "    "
	for _, assign := range tool.Assignments {
		sb.WriteString("\n")
		sb.WriteString(indent)
		sb.WriteString(assign.Name)
		sb.WriteString("=")
		if assign.Name == "invoke" {
			sb.WriteString(invokeRef)
		} else {
			sb.WriteString(assignmentValueSource(assign, indent))
		}
		sb.WriteString(",")
	}
	if len(tool.Assignments) > 0 {
		sb.WriteString("\n")
	}
	sb.WriteString(")")

	return wrapWithMetaIfNeeded(sb.String(), tool.Annotations, "    ", "")
}

// toolImports returns the collected Python imports from dotted-path tool invokes.
func (b *LangGraphBackend) toolImports() []python.PythonImport {
	var imports []python.PythonImport
	for _, resolved := range b.resolvedTools {
		if resolved.pyImport != nil {
			imports = append(imports, *resolved.pyImport)
		}
	}
	return imports
}

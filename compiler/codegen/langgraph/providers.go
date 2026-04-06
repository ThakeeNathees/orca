// Package langgraph implements Python/LangGraph code generation from the IR.
package langgraph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/codegen"
	"github.com/thakee/orca/compiler/codegen/python"
	"github.com/thakee/orca/compiler/diagnostic"
)

// providerInfo holds LangChain metadata for a model provider.
type providerInfo struct {
	PyImport python.PythonImport // import line + pip package for dependency resolution
}

// providerRegistry maps provider names to their LangChain metadata.
var providerRegistry = map[string]providerInfo{
	"openai": {
		PyImport: python.PythonImport{
			Module:     "langchain_openai",
			Package:    "langchain-openai",
			FromImport: true,
			Symbols:    []python.ImportSymbol{{Name: "ChatOpenAI"}},
		},
	},
	"anthropic": {
		PyImport: python.PythonImport{
			Module:     "langchain_anthropic",
			Package:    "langchain-anthropic",
			FromImport: true,
			Symbols:    []python.ImportSymbol{{Name: "ChatAnthropic"}},
		},
	},
	"google": {
		PyImport: python.PythonImport{
			Module:     "langchain_google_genai",
			Package:    "langchain-google-genai",
			FromImport: true,
			Symbols:    []python.ImportSymbol{{Name: "ChatGoogleGenerativeAI"}},
		},
	},
}

// providerClassName returns the LangChain class name for a provider string,
// e.g. "openai" → "ChatOpenAI". Returns empty string if unknown.
func providerClassName(provider string) string {
	info, ok := providerRegistry[provider]
	if !ok || len(info.PyImport.Symbols) == 0 {
		return ""
	}
	return info.PyImport.Symbols[0].Name
}

// resolvedProviders holds the result of a single pass over model blocks:
// sorted, deduplicated provider Python imports. Pip dependencies are derived
// from these via dependenciesFromPythonImports.
type resolvedProviders struct {
	providerImports []python.PythonImport
}

// dependenciesFromProviders builds codegen.Dependency values for the lockfile:
// always langchain-core, then unique non-empty PyImport.Package names sorted.
// The hasWorkflows flag adds langgraph and langchain as base framework deps.
func dependenciesFromProviders(resolvedProviders resolvedProviders, hasWorkflows bool) []codegen.Dependency {
	deps := []codegen.Dependency{{Name: "langchain-core"}}
	if hasWorkflows {
		deps = append(deps,
			codegen.Dependency{Name: "langchain"},
			codegen.Dependency{Name: "langgraph"},
		)
	}
	seen := make(map[string]bool)
	var pipPkgs []string
	for _, imp := range resolvedProviders.providerImports {
		if imp.Package != "" && !seen[imp.Package] {
			seen[imp.Package] = true
			pipPkgs = append(pipPkgs, imp.Package)
		}
	}
	sort.Strings(pipPkgs)
	for _, pkg := range pipPkgs {
		deps = append(deps, codegen.Dependency{Name: pkg})
	}
	return deps
}

// resolveProviders walks all model block bodies (top-level and inline) to
// collect known providers and emit diagnostics for unknown ones.
func (b *LangGraphBackend) resolveProviders() {
	seen := make(map[string]bool)
	var names []string

	for _, body := range b.collectBodiesByKind(analyzer.BlockKindModel) {
		expr, ok := body.GetFieldExpression("provider")
		if !ok {
			continue
		}

		v, d := analyzer.ConstFold(expr, b.Program)
		b.Program.Diagnostics = append(b.Program.Diagnostics, d...)

		if v.Kind != analyzer.ConstString {
			continue
		}

		if _, known := providerRegistry[v.Str]; !known {
			b.Program.Diagnostics = append(b.Program.Diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeUnknownProvider,
				Position: diagnostic.Position{Line: expr.Start().Line, Column: expr.Start().Column},
				Message:  fmt.Sprintf("unknown provider %q", v.Str),
				Source:   "codegen",
			})
			continue
		}

		if !seen[v.Str] {
			seen[v.Str] = true
			names = append(names, v.Str)
		}
	}
	sort.Strings(names)

	var imports []python.PythonImport
	for _, p := range names {
		if info, ok := providerRegistry[p]; ok {
			imports = append(imports, info.PyImport)
		}
	}

	b.resolvedProviders = resolvedProviders{providerImports: imports}
}

// writeModelSection emits model blocks with the provider field replaced by the
// resolved LangChain class reference. Instead of provider="openai", emits
// provider_class=ChatOpenAI so the runtime can instantiate directly.
func (b *LangGraphBackend) writeModelSection(s *strings.Builder) {
	models := b.CollectBlocksByKind(analyzer.AnnotationTriggerNode)
	if len(models) == 0 {
		return
	}

	s.WriteString("\n# --- Models ---\n")

	for _, model := range models {
		s.WriteString("\n")
		fmt.Fprintf(s, "%s = %s\n", model.Name, modelBlockSource(model))
	}
}

// modelBlockSource generates the __orca_model(...) call with the provider field
// substituted from a string to the resolved class name.
func modelBlockSource(model *ast.BlockStatement) string {
	var sb strings.Builder
	sb.WriteString(orcaPrefix + "model(")

	indent := "    "
	for _, assign := range model.Assignments {
		sb.WriteString("\n")
		sb.WriteString(indent)

		if assign.Name == "provider" {
			if str, ok := assign.Value.(*ast.StringLiteral); ok {
				if className := providerClassName(str.Value); className != "" {
					sb.WriteString("provider_class=")
					sb.WriteString(className)
					sb.WriteString(",")
					continue
				}
			}
		}

		sb.WriteString(assign.Name)
		sb.WriteString("=")
		sb.WriteString(assignmentValueSource(assign, indent))
		sb.WriteString(",")
	}
	if len(model.Assignments) > 0 {
		sb.WriteString("\n")
	}
	sb.WriteString(")")

	return wrapWithMetaIfNeeded(sb.String(), model.Annotations, "    ", "")
}

// collectBodiesByKind returns all BlockBody nodes matching the given kind,
// including both top-level block statements and inline block expressions
// nested within other blocks' assignments.
func (b *LangGraphBackend) collectBodiesByKind(kind string) []*ast.BlockBody {
	var bodies []*ast.BlockBody
	for _, stmt := range b.Program.Ast.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		if block.Kind == kind {
			bodies = append(bodies, &block.BlockBody)
		}
		collectInlineBodies(block.Assignments, kind, &bodies)
	}
	return bodies
}

// collectInlineBodies recursively collects BlockBody nodes of the given kind
// from inline BlockExpression nodes nested within assignment values.
func collectInlineBodies(assignments []*ast.Assignment, kind string, bodies *[]*ast.BlockBody) {
	for _, assign := range assignments {
		if assign == nil {
			continue
		}
		if be, ok := assign.Value.(*ast.BlockExpression); ok {
			if be.Kind == kind {
				*bodies = append(*bodies, &be.BlockBody)
			}
			collectInlineBodies(be.Assignments, kind, bodies)
		}
	}
}

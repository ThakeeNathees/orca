// Package langgraph implements Python/LangGraph code generation from the IR.
package langgraph

import (
	"sort"

	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/codegen"
	"github.com/thakee/orca/compiler/codegen/python"
	"github.com/thakee/orca/compiler/token"
)

// providerInfo holds LangChain metadata for a model provider.
type providerInfo struct {
	PyImport python.PythonImport // import line + pip package for dependency resolution
	Class    string              // e.g. "ChatOpenAI"
}

// providers maps provider names to their LangChain metadata.
var providerRegistry = map[string]providerInfo{
	"openai": {
		PyImport: python.PythonImport{
			Module:     "langchain_openai",
			Package:    "langchain-openai",
			FromImport: true,
			Symbols:    []python.ImportSymbol{{Name: "ChatOpenAI"}},
		},
		Class: "ChatOpenAI",
	},
	"anthropic": {
		PyImport: python.PythonImport{
			Module:     "langchain_anthropic",
			Package:    "langchain-anthropic",
			FromImport: true,
			Symbols:    []python.ImportSymbol{{Name: "ChatAnthropic"}},
		},
		Class: "ChatAnthropic",
	},
	"google": {
		PyImport: python.PythonImport{
			Module:     "langchain_google_genai",
			Package:    "langchain-google-genai",
			FromImport: true,
			Symbols:    []python.ImportSymbol{{Name: "ChatGoogleGenerativeAI"}},
		},
		Class: "ChatGoogleGenerativeAI",
	},
}

// resolvedProviders holds the result of a single pass over model blocks:
// sorted, deduplicated provider Python imports. Pip dependencies are derived
// from these via dependenciesFromPythonImports.
type resolvedProviders struct {
	providerImports []python.PythonImport
}

// dependenciesFromProviders builds codegen.Dependency values for the lockfile:
// always langchain-core, then unique non-empty PyImport.Package names sorted.
func dependenciesFromProviders(resolvedProviders resolvedProviders) []codegen.Dependency {
	deps := []codegen.Dependency{{Name: "langchain-core"}}
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

// resolveProviders walks model blocks once to collect known providers and
// emit diagnostics for unknown providers (during writeModel).
func (b *LangGraphBackend) resolveProviders() {
	seen := make(map[string]bool)
	var names []string

	for _, block := range b.CollectBlocks(token.MODEL) {
		expr, ok := block.GetFieldExpression("provider")
		if !ok {
			continue
		}

		// Const fold the expression.
		v, d := analyzer.ConstFold(expr, b.Program)
		b.Program.Diagnostics = append(b.Program.Diagnostics, d...)

		// If v is not a string value, we set diagnostics that langgraph
		// provider should be a compile-time constant.
		if v.Kind != analyzer.ConstString {
			continue
		}

		// Only known providers contribute to imports/deps; unknown providers are
		// diagnosed during model codegen (writeModel) to avoid duplicate errors.
		if _, known := providerRegistry[v.Str]; !known {
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

// Package langgraph implements Python/LangGraph code generation from the IR.
package langgraph

import (
	"sort"

	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/codegen"
	"github.com/thakee/orca/compiler/token"
)

// providerInfo holds LangChain metadata for a model provider.
type providerInfo struct {
	Import string // e.g. "from langchain_openai import ChatOpenAI"
	Class  string // e.g. "ChatOpenAI"
	Dep    string // pip package name, e.g. "langchain-openai"
}

// providers maps provider names to their LangChain metadata.
var providerRegistry = map[string]providerInfo{
	"openai": {
		Import: "from langchain_openai import ChatOpenAI",
		Class:  "ChatOpenAI",
		Dep:    "langchain-openai",
	},
	"anthropic": {
		Import: "from langchain_anthropic import ChatAnthropic",
		Class:  "ChatAnthropic",
		Dep:    "langchain-anthropic",
	},
	"google": {
		Import: "from langchain_google_genai import ChatGoogleGenerativeAI",
		Class:  "ChatGoogleGenerativeAI",
		Dep:    "langchain-google-genai",
	},
}

// resolvedProviders holds the result of a single pass over model blocks,
// extracting known providers and building pip dependencies.
type resolvedProviders struct {
	providers    []string             // sorted, deduplicated known provider names
	dependencies []codegen.Dependency // langchain-core + provider-specific deps
}

// resolveProviders walks model blocks once to collect known providers,
// build the dependency list, and emit diagnostics for unknown providers.
func (b *LangGraphBackend) resolveProviders() resolvedProviders {
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

	deps := []codegen.Dependency{{Name: "langchain-core"}}
	seenDeps := make(map[string]bool)
	var pipPkgs []string
	for _, p := range names {
		if info, ok := providerRegistry[p]; ok && !seenDeps[info.Dep] {
			pipPkgs = append(pipPkgs, info.Dep)
			seenDeps[info.Dep] = true
		}
	}
	sort.Strings(pipPkgs)
	for _, pkg := range pipPkgs {
		deps = append(deps, codegen.Dependency{Name: pkg})
	}

	return resolvedProviders{providers: names, dependencies: deps}
}

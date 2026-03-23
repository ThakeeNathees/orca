// Package ir defines the intermediate representation produced by the analyzer
// and consumed by codegen. The IR is fully resolved — no references, no symbol
// tables. Codegen can walk it linearly and emit code without lookups.
package ir

import "github.com/thakee/orca/compiler/ast"

// IR is the top-level intermediate representation of an Orca program.
// All references are resolved to concrete values.
type IR struct {
	Lets      []LetVar
	Models    []ModelDef
	Agents    []AgentDef
	Providers []string // unique provider names, sorted
}

// LetVar is a resolved variable from a let block.
// Keeps the AST expression so codegen can emit arbitrary Python expressions.
type LetVar struct {
	Name  string
	Value ast.Expression

	// Source location for source mapping.
	File   string
	Line   int
	Column int
}

// ModelDef is a fully resolved model block.
type ModelDef struct {
	Name        string
	Provider    string  // "openai", "anthropic", etc.
	ModelName   string  // "gpt-4o", "claude-3-opus", etc.
	Temperature float64 // 0.0 if not set
	HasTemp     bool    // true if temperature was explicitly set

	// Source location for source mapping.
	File   string // the .oc file this block came from
	Line   int
	Column int
}

// AgentDef is a fully resolved agent block.
type AgentDef struct {
	Name    string
	Model   string   // resolved model name (e.g. "gpt4")
	Persona string
	Tools   []string // resolved tool names

	// Source location for source mapping.
	File   string
	Line   int
	Column int
}

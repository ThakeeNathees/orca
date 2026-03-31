// Package codegen defines shared types and interfaces for code generation.
// Language-specific backends live in sub-packages (e.g., python/langgraph).
package codegen

import (
	"fmt"
	"sort"

	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/token"
)

// Dependency represents a package dependency required by the generated code.
type Dependency struct {
	Name          string
	Version       string
	DevDependency bool
}

// CodegenOutput holds the complete output from a code generation backend.
type CodegenOutput struct {
	RootDir      OutputDirectory
	Dependencies []Dependency
	Diagnostics  []diagnostic.Diagnostic
}

// OutputDirectory represents a directory in the generated output tree.
type OutputDirectory struct {
	Name        string
	Files       []OutputFile
	Directories []OutputDirectory
}

// OutputFile represents a single generated file.
type OutputFile struct {
	Name    string
	Content string
}

// CodegenBackend defines a code generation target. Each backend (LangGraph, etc.)
// implements this interface to produce output from the analyzed AST.
type CodegenBackend interface {
	Generate() CodegenOutput
}

// BaseBackend provides common functionality shared across all codegen backends.
// Embed this in concrete backend types to get access to block collection helpers.
type BaseBackend struct {
	Program analyzer.AnalyzedProgram
}

// CollectBlocks returns all block statements of the given token type.
func (b *BaseBackend) CollectBlocks(tokenType token.TokenType) []*ast.BlockStatement {
	var blocks []*ast.BlockStatement
	for _, stmt := range b.Program.Ast.Statements {
		if block, ok := stmt.(*ast.BlockStatement); ok && block.TokenStart.Type == tokenType {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// CollectLets returns all let block statements.
func (b *BaseBackend) CollectLets() []*ast.BlockStatement {
	return b.CollectBlocks(token.LET)
}

// CollectProviders returns sorted, unique provider names from model blocks.
func (b *BaseBackend) CollectProviders() []string {
	seen := make(map[string]bool)
	for _, block := range b.CollectBlocks(token.MODEL) {
		for _, assign := range block.Assignments {
			if assign.Name == "provider" {
				if s, ok := assign.Value.(*ast.StringLiteral); ok {
					seen[s.Value] = true
				}
			}
		}
	}
	var providers []string
	for p := range seen {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	return providers
}

// SourceComment formats a source mapping comment like "agents.oc:42" or "line 42".
func SourceComment(file string, line int) string {
	if file != "" {
		return fmt.Sprintf("%s:%d", file, line)
	}
	return fmt.Sprintf("line %d", line)
}

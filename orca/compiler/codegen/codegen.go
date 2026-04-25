// Package codegen defines shared types and interfaces for code generation.
// Language-specific backends live in sub-packages (e.g., python/langgraph).
package codegen

import (
	"github.com/thakee/orca/orca/compiler/analyzer"
	"github.com/thakee/orca/orca/compiler/ast"
	"github.com/thakee/orca/orca/compiler/diagnostic"
)

// CodegenOutput holds the complete output from a code generation backend.
type CodegenOutput struct {
	BackendType  BackendType
	Dependencies []Dependency
	RootDir      OutputDirectory
	Diagnostics  []diagnostic.Diagnostic
}

// BackendType identifies which code generation backend produced a given output.
//
// This is used by higher-level orchestration (e.g. `orca run`) to pick the
// appropriate runner without hardcoding backend-specific behavior into the CLI.
type BackendType string

const (
	BackendLangGraph BackendType = "langgraph"
	// These backends are not yet implemented (nor will be in the near future).
	BackendCrewAI  BackendType = "crewai"
	BackendAutogen BackendType = "autogen"
)

// Dependency represents a package dependency required by the generated code.
type Dependency struct {
	Name          string
	MinVersion    string
	DevDependency bool
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

// CollectBlocksByKind returns all block statements of the given block kind.
func (b *BaseBackend) CollectBlocksByKind(kind string) []*ast.BlockStatement {
	var blocks []*ast.BlockStatement
	for _, stmt := range b.Program.Ast.Statements {
		if block, ok := stmt.(*ast.BlockStatement); ok && block.Kind == kind {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// BlockByName returns the block statement with the given name, or nil if not found.
func (b *BaseBackend) BlockByName(name string) *ast.BlockStatement {
	for _, stmt := range b.Program.Ast.Statements {
		if block, ok := stmt.(*ast.BlockStatement); ok && block.Name == name {
			return block
		}
	}
	return nil
}

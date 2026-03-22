// Package codegen generates target code from an analyzed AST.
// Currently targets LangGraph Python as the sole backend, but the
// architecture supports adding other backends in the future.
package codegen

import "github.com/thakee/orca/compiler/ast"

// Generate produces target code from the given AST program.
// Returns the generated source as a string.
// TODO: implement Python/LangGraph code generation with source mapping.
func Generate(program *ast.Program) string {
	return ""
}

// Package evaluator will transform the AST into an intermediate representation
// (JSON/YAML IR) and eventually generate Python code targeting LangGraph.
// Currently a stub awaiting parser completion.
package evaluator

import "github.com/thakee/orca/ast"

// Eval walks the AST and produces the evaluated result.
// TODO: implement IR generation from AST nodes and Python codegen.
func Eval(node ast.Node) any {
	return nil
}

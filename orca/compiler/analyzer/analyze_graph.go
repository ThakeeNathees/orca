package analyzer

import (
	"fmt"

	"github.com/thakee/orca/orca/compiler/ast"
	"github.com/thakee/orca/orca/compiler/diagnostic"
	"github.com/thakee/orca/orca/compiler/graph"
)

// Graph related analysis functions / helpers.

// buildBlockDependencyGraph constructs a directed graph of block-to-block
// dependencies by walking each block's expressions for references to other
// user-defined blocks. The graph is topologically sorted to produce a valid
// emission order for codegen. Cycles are reported as diagnostics.
func buildBlockDependencyGraph(ap *AnalyzedProgram) {
	g := graph.New[string]()

	// Collect user-defined block names (everything not from bootstrap).
	userBlocks := make(map[string]*ast.BlockStatement)
	for _, stmt := range ap.Ast.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		userBlocks[block.Name] = block
		g.AddNode(block.Name)
	}

	// Extract dependencies: for each block, walk its body and find
	// references to other user-defined blocks.
	for _, stmt := range ap.Ast.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		deps := make(map[string]bool)
		for _, assign := range block.BlockBody.Assignments {
			if assign.Value != nil {
				collectBlockDeps(assign.Value, userBlocks, deps)
			}
		}
		for _, expr := range block.BlockBody.Expressions {
			collectBlockDeps(expr, userBlocks, deps)
		}
		for dep := range deps {
			if dep != block.Name { // skip self-references (handled by other checks)
				g.AddEdge(dep, block.Name)
			}
		}
	}

	// Topological sort — reverse because edges point from dependent → dependency,
	// so dependencies must be emitted first.
	sorted, err := g.TopologicalSort()
	if err != nil {
		// Report cycle diagnostic on each block involved.
		// Since we can't easily pinpoint exactly which blocks form the cycle,
		// report on all blocks that weren't emitted by the sort.
		ap.Diagnostics = append(ap.Diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     diagnostic.CodeCyclicDependency,
			Position: diagnostic.Position{Line: 1, Column: 1},
			Message:  fmt.Sprintf("block dependency cycle detected: %s", err),
			Source:   "analyzer",
		})
		// Fall back to source order when there's a cycle.
		ap.BlockOrder = g.Nodes()
		return
	}

	ap.BlockOrder = sorted
}

// collectBlockDeps recursively walks an expression and collects the names of
// any user-defined blocks it references.
func collectBlockDeps(expr ast.Expression, userBlocks map[string]*ast.BlockStatement, deps map[string]bool) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		if _, ok := userBlocks[e.Value]; ok {
			deps[e.Value] = true
		}
	case *ast.MemberAccess:
		// The dependency is on the root object, not the member.
		collectBlockDeps(e.Object, userBlocks, deps)
	case *ast.BinaryExpression:
		collectBlockDeps(e.Left, userBlocks, deps)
		collectBlockDeps(e.Right, userBlocks, deps)
	case *ast.ListLiteral:
		for _, elem := range e.Elements {
			collectBlockDeps(elem, userBlocks, deps)
		}
	case *ast.MapLiteral:
		for _, entry := range e.Entries {
			collectBlockDeps(entry.Key, userBlocks, deps)
			collectBlockDeps(entry.Value, userBlocks, deps)
		}
	case *ast.CallExpression:
		collectBlockDeps(e.Callee, userBlocks, deps)
		for _, arg := range e.Arguments {
			collectBlockDeps(arg, userBlocks, deps)
		}
	case *ast.Subscription:
		collectBlockDeps(e.Object, userBlocks, deps)
		for _, idx := range e.Indices {
			collectBlockDeps(idx, userBlocks, deps)
		}
	case *ast.TernaryExpression:
		collectBlockDeps(e.Condition, userBlocks, deps)
		collectBlockDeps(e.TrueExpr, userBlocks, deps)
		collectBlockDeps(e.FalseExpr, userBlocks, deps)
	case *ast.Lambda:
		// Lambda params shadow outer names, but the body may still reference
		// outer blocks. We don't exclude param names from deps because
		// param names are not block names.
		for _, p := range e.Params {
			collectBlockDeps(p.TypeExpr, userBlocks, deps)
		}
		if e.ReturnType != nil {
			collectBlockDeps(e.ReturnType, userBlocks, deps)
		}
		collectBlockDeps(e.Body, userBlocks, deps)
	case *ast.BlockExpression:
		for _, assign := range e.BlockBody.Assignments {
			if assign.Value != nil {
				collectBlockDeps(assign.Value, userBlocks, deps)
			}
		}
		for _, subExpr := range e.BlockBody.Expressions {
			collectBlockDeps(subExpr, userBlocks, deps)
		}
	}
}

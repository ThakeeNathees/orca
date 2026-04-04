// Package workflow provides graph resolution utilities for Orca workflow blocks.
// It extracts edges and node names from AST expressions, shared by both the
// analyzer (validation) and codegen (code generation) stages.
package workflow

import (
	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

// Graph terminal node names. These are virtual nodes representing the
// start and end of a workflow graph — not user-defined blocks.
const (
	NodeSTART = "START"
	NodeEND   = "END"
)

// Edge represents a single directed edge between two named nodes
// in a workflow graph, extracted from arrow expressions.
type Edge struct {
	From string
	To   string
}

// ResolvedWorkflow holds the extracted graph structure for a single workflow block.
type ResolvedWorkflow struct {
	Name  string
	Nodes []string // unique node names (excluding START/END), in order of first appearance
	Edges []Edge   // all edges in declaration order, including inferred START/END
}

// Resolve extracts nodes and edges from a workflow block's arrow expressions.
// After collecting explicit edges, it infers implicit START/END connections:
//   - Any node with no incoming edges gets an implicit START -> node edge
//   - Any node with no outgoing edges gets an implicit node -> END edge
func Resolve(block *ast.BlockStatement) ResolvedWorkflow {
	rw := ResolvedWorkflow{Name: block.Name}
	seenNodes := make(map[string]bool)
	seenEdges := make(map[[2]string]bool)

	for _, expr := range block.Expressions {
		edges := EdgesFromExpr(expr)
		for _, e := range edges {
			edgeKey := [2]string{e.From, e.To}
			if seenEdges[edgeKey] {
				continue
			}
			seenEdges[edgeKey] = true
			rw.Edges = append(rw.Edges, e)

			for _, name := range []string{e.From, e.To} {
				if name != NodeSTART && name != NodeEND && !seenNodes[name] {
					seenNodes[name] = true
					rw.Nodes = append(rw.Nodes, name)
				}
			}
		}
	}

	rw.Edges = inferTerminals(rw.Nodes, rw.Edges)
	return rw
}

// inferTerminals adds implicit START/END edges for nodes that have no
// incoming or outgoing connections. Explicit START/END edges are preserved.
func inferTerminals(nodes []string, edges []Edge) []Edge {
	hasIncoming := make(map[string]bool)
	hasOutgoing := make(map[string]bool)

	for _, e := range edges {
		if e.To != NodeEND {
			hasIncoming[e.To] = true
		}
		if e.From != NodeSTART {
			hasOutgoing[e.From] = true
		}
	}

	// Prepend implicit START edges (before existing edges for natural ordering).
	var startEdges []Edge
	for _, node := range nodes {
		if !hasIncoming[node] {
			startEdges = append(startEdges, Edge{From: NodeSTART, To: node})
		}
	}

	// Append implicit END edges.
	var endEdges []Edge
	for _, node := range nodes {
		if !hasOutgoing[node] {
			endEdges = append(endEdges, Edge{From: node, To: NodeEND})
		}
	}

	result := make([]Edge, 0, len(startEdges)+len(edges)+len(endEdges))
	result = append(result, startEdges...)
	result = append(result, edges...)
	result = append(result, endEdges...)
	return result
}

// EdgesFromExpr walks a (possibly chained) arrow expression and returns
// the list of individual edges. For example, A -> B -> C yields:
// [{A, B}, {B, C}]. Non-arrow expressions return nil.
func EdgesFromExpr(expr ast.Expression) []Edge {
	bin, ok := expr.(*ast.BinaryExpression)
	if !ok || bin.Operator.Type != token.ARROW {
		return nil
	}

	// The parser builds left-associative trees: ((A -> B) -> C).
	// Recursively flatten the left side, then connect its last node to the right.
	leftEdges := EdgesFromExpr(bin.Left)

	rightName := ExprToNodeName(bin.Right)

	if len(leftEdges) > 0 {
		lastTo := leftEdges[len(leftEdges)-1].To
		return append(leftEdges, Edge{From: lastTo, To: rightName})
	}

	leftName := ExprToNodeName(bin.Left)
	return []Edge{{From: leftName, To: rightName}}
}

// ExprToNodeName returns the workflow node name for an expression.
// Returns empty string for unrecognized expression types.
func ExprToNodeName(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Value
	case *ast.MemberAccess:
		return ExprToNodeName(e.Object) + "." + e.Member
	case *ast.Subscription:
		return ExprToNodeName(e.Object) + "[" + ExprToNodeName(e.Index) + "]"
	}
	// TODO: Return a unique string.
	return ""
}

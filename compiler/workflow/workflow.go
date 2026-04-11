// Package workflow provides graph resolution utilities for Orca workflow blocks.
// It extracts edges and node names from AST expressions, shared by both the
// analyzer (validation) and codegen (code generation) stages.
package workflow

import (
	"fmt"
	"strings"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/graph"
	"github.com/thakee/orca/compiler/token"
)

// inlineBlockCounter generates unique synthetic names for inline block expressions
// (e.g. inline branch, inline agent, etc.) used as workflow nodes.
var inlineBlockCounter int64

const (
	// Graph terminal node names. These are virtual nodes representing the
	// start and end of a workflow graph — not user-defined blocks.
	NodeSTART = "START"
	NodeEND   = "END"

	// BlockKindBranch is the block kind for branch nodes.
	BlockKindBranch = "branch"
)

// Edge represents a single directed edge between two named nodes
// in a workflow graph, extracted from arrow expressions.
type Edge struct {
	From string
	To   string
}

// BranchRoute represents a single route in a branch's route table.
// The key is a runtime expression (string, number, boolean) and the value
// is a workflow graph expression (can be a single node or a complex subgraph).
type BranchRoute struct {
	Key        ast.Expression // route key expression (string, number, bool, or identifier "default")
	EntryNodes []string       // first nodes in this route's subgraph (conditional edge targets)
	Nodes      []string       // all processing nodes in this route's subgraph
	Edges      []Edge         // all edges within this route's subgraph
}

// Branch represents a conditional routing point in a workflow graph.
// It replaces a simple edge with conditional edges based on the transform
// output matching route keys.
type Branch struct {
	Name      string         // synthetic name for inline branches, block name for named
	Preds     []string       // predecessor node names (who feeds into this branch)
	Transform ast.Expression // optional transform lambda applied to predecessor output
	Routes    []BranchRoute  // route table entries
	Body      *ast.BlockBody // the branch block body (for codegen access)
}

// ResolvedWorkflow holds the extracted graph structure for a single workflow block.
type ResolvedWorkflow struct {
	Name       string
	Nodes      []string            // processing node names (triggers excluded), in order of first appearance
	Edges      []Edge              // edges between processing nodes + inferred END edges (no START edges)
	EntryNodes []string            // processing nodes with no incoming edges from other processing nodes
	Triggers   []string            // trigger node names in order of first appearance
	TriggerMap map[string][]string // trigger name → processing entry node names it connects to
	Branches   []Branch            // branch nodes with conditional routing
}

// HasTriggers returns true if the workflow has any trigger nodes.
func (rw *ResolvedWorkflow) HasTriggers() bool {
	return len(rw.Triggers) > 0
}

// Predecessors returns the list of predecessor processing node names for the
// given node. Entry nodes (no incoming edges) return an empty slice.
// For nodes that are route entry points of a branch, the branch's predecessors
// are included (since the branch routes input from its predecessors to route targets).
func (rw *ResolvedWorkflow) Predecessors(node string) []string {
	var preds []string
	for _, e := range rw.Edges {
		if e.To == node && e.From != NodeSTART && e.From != NodeEND {
			preds = append(preds, e.From)
		}
	}
	// Check if this node is a route entry point — if so, add the branch's predecessors.
	for _, branch := range rw.Branches {
		for _, route := range branch.Routes {
			for _, entry := range route.EntryNodes {
				if entry == node {
					preds = append(preds, branch.Preds...)
				}
			}
		}
	}
	return preds
}

// TODO: Consider the workflow router is always returns list[string] and if there
// is a single entry node, the length will be 1.
//
// IsFanOut returns true if any trigger connects to multiple entry nodes.
func (rw *ResolvedWorkflow) IsFanOut() bool {
	for _, entries := range rw.TriggerMap {
		if len(entries) > 1 {
			return true
		}
	}
	return false
}

// Resolve extracts nodes and edges from a workflow block's arrow expressions.
// The isTrigger predicate identifies which nodes are triggers (cron, webhook).
// The getBranchBody predicate identifies which named nodes are branch blocks.
// Pass nil for either if no classification is needed.
//
// Triggers are separated from processing nodes: they appear in Triggers/TriggerMap
// but not in Nodes/Edges. Branch nodes are extracted into Branches and their
// route tables are expanded into the processing graph.
// Implicit END edges are added for processing nodes with no outgoing edges.
func Resolve(block *ast.BlockStatement, isTrigger func(string) bool, getBranchBody func(string) *ast.BlockBody) ResolvedWorkflow {
	if isTrigger == nil {
		isTrigger = func(string) bool { return false }
	}
	if getBranchBody == nil {
		getBranchBody = func(string) *ast.BlockBody { return nil }
	}

	rw := ResolvedWorkflow{
		Name:       block.Name,
		TriggerMap: make(map[string][]string),
	}

	// Build a graph of all nodes and edges from arrow expressions.
	// Triggers are tracked separately and excluded from the processing graph.
	allGraph := graph.New[string]()
	triggers := make(map[string]bool)

	// classifyNode registers a node as either a trigger or processing node.
	classifyNode := func(name string) {
		if isTrigger(name) {
			if !triggers[name] {
				triggers[name] = true
				rw.Triggers = append(rw.Triggers, name)
			}
		}
		allGraph.AddNode(name)
	}

	for _, expr := range block.Expressions {
		edges := EdgesFromExpr(expr)
		if len(edges) == 0 {
			if name := ExprToNodeName(expr); name != "" {
				classifyNode(name)
			}
			continue
		}
		for _, e := range edges {
			classifyNode(e.From)
			classifyNode(e.To)
			allGraph.AddEdge(e.From, e.To)
		}
	}

	// Identify branch nodes and cache their bodies. Uses allGraph.Nodes()
	// for deterministic iteration order (insertion order).
	branchNames := make(map[string]bool)
	branchBodies := make(map[string]*ast.BlockBody)
	for _, name := range allGraph.Nodes() {
		if body := getBranchBody(name); body != nil {
			branchNames[name] = true
			branchBodies[name] = body
		}
	}

	// Build a predecessor map in a single pass over all edges.
	branchPreds := make(map[string][]string)
	for _, e := range allGraph.Edges() {
		if branchNames[e.To] && !triggers[e.From] && !branchNames[e.From] {
			branchPreds[e.To] = append(branchPreds[e.To], e.From)
		}
	}

	// Extract branch metadata in deterministic order.
	for _, branchName := range allGraph.Nodes() {
		body, ok := branchBodies[branchName]
		if !ok {
			continue
		}
		branch := Branch{
			Name:  branchName,
			Body:  body,
			Preds: branchPreds[branchName],
		}

		if transformExpr, ok := body.GetFieldExpression("transform"); ok {
			branch.Transform = transformExpr
		}

		if routeExpr, ok := body.GetFieldExpression("route"); ok {
			if mapLit, ok := routeExpr.(*ast.MapLiteral); ok {
				for _, entry := range mapLit.Entries {
					branch.Routes = append(branch.Routes, extractBranchRoute(entry))
				}
			}
		}

		rw.Branches = append(rw.Branches, branch)
	}

	// Separate triggers and branches from processing nodes.
	procGraph := graph.New[string]()
	for _, name := range allGraph.Nodes() {
		if !triggers[name] && !branchNames[name] {
			rw.Nodes = append(rw.Nodes, name)
			procGraph.AddNode(name)
		}
	}

	// Add route chain nodes and edges to the processing graph.
	// Pred→entry edges are "conditional" — they exist in procGraph for leaf detection
	// but are excluded from rw.Edges (codegen emits them via add_conditional_edges).
	var conditionalEdges map[Edge]bool
	for _, branch := range rw.Branches {
		for _, route := range branch.Routes {
			for _, node := range route.Nodes {
				if !procGraph.HasNode(node) {
					rw.Nodes = append(rw.Nodes, node)
				}
				procGraph.AddNode(node)
			}
			for _, e := range route.Edges {
				procGraph.AddEdge(e.From, e.To)
			}
			for _, pred := range branch.Preds {
				for _, entry := range route.EntryNodes {
					if conditionalEdges == nil {
						conditionalEdges = make(map[Edge]bool)
					}
					conditionalEdges[Edge{From: pred, To: entry}] = true
					procGraph.AddEdge(pred, entry)
				}
			}
		}
	}

	// Separate trigger edges from processing edges and build TriggerMap.
	// Skip edges involving branch nodes (they're handled by conditional routing).
	for _, e := range allGraph.Edges() {
		if branchNames[e.From] || branchNames[e.To] {
			continue
		}
		if triggers[e.From] {
			rw.TriggerMap[e.From] = append(rw.TriggerMap[e.From], e.To)
		} else {
			procGraph.AddEdge(e.From, e.To)
		}
	}

	// Standalone triggers (not connected via arrows) implicitly connect
	// to all processing nodes (which will become entry nodes).
	for _, trig := range rw.Triggers {
		if _, mapped := rw.TriggerMap[trig]; !mapped && len(rw.Nodes) > 0 {
			rw.TriggerMap[trig] = append([]string{}, rw.Nodes...)
		}
	}

	// Infer END edges for leaf processing nodes (no outgoing edges).
	// Skip conditional edges (pred→route entry) — codegen handles those separately.
	for _, e := range procGraph.Edges() {
		edge := Edge{From: e.From, To: e.To}
		if conditionalEdges[edge] {
			continue
		}
		rw.Edges = append(rw.Edges, edge)
	}
	for _, leaf := range procGraph.LeafNodes() {
		rw.Edges = append(rw.Edges, Edge{From: leaf, To: NodeEND})
	}

	// Entry nodes: processing nodes with no incoming edges.
	rw.EntryNodes = procGraph.EntryNodes()

	return rw
}

// extractBranchRoute extracts a BranchRoute from a map entry.
// The key is kept as an ast.Expression (can be string, number, bool, or "default").
// The value is a workflow graph expression: a single node, a chain, or multiple
// expressions forming a complex subgraph.
func extractBranchRoute(entry ast.MapEntry) BranchRoute {
	route := BranchRoute{
		Key: entry.Key,
	}

	// Build a subgraph from the route value expression.
	subgraph := graph.New[string]()
	var walkExpr func(expr ast.Expression)
	walkExpr = func(expr ast.Expression) {
		edges := EdgesFromExpr(expr)
		if len(edges) == 0 {
			if name := ExprToNodeName(expr); name != "" {
				subgraph.AddNode(name)
			}
			return
		}
		for _, e := range edges {
			subgraph.AddNode(e.From)
			subgraph.AddNode(e.To)
			subgraph.AddEdge(e.From, e.To)
		}
	}
	walkExpr(entry.Value)

	route.Nodes = subgraph.Nodes()
	route.EntryNodes = subgraph.EntryNodes()
	for _, e := range subgraph.Edges() {
		route.Edges = append(route.Edges, Edge{From: e.From, To: e.To})
	}

	return route
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
// For inline BlockExpression nodes, generates a synthetic name like "__branch_0".
// Returns empty string for unrecognized expression types.
func ExprToNodeName(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Value
	case *ast.MemberAccess:
		return ExprToNodeName(e.Object) + "." + e.Member
	case *ast.Subscription:
		var indices []string
		for _, idx := range e.Indices {
			indices = append(indices, ExprToNodeName(idx))
		}
		return ExprToNodeName(e.Object) + "[" + strings.Join(indices, ", ") + "]"
	case *ast.BlockExpression:
		// The type system sets BlockNameAnon during analysis. If it's still
		// empty (e.g. in tests), generate a synthetic name as fallback.
		if e.BlockNameAnon == "" {
			inlineBlockCounter++
			e.BlockNameAnon = fmt.Sprintf("__%s_%d", e.Kind, inlineBlockCounter-1)
		}
		return e.BlockNameAnon
	}
	return ""
}

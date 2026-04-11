// Package workflow provides graph resolution utilities for Orca workflow blocks.
// It extracts edges and node names from AST expressions, shared by both the
// analyzer (validation) and codegen (code generation) stages.
package workflow

import (
	"strings"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/graph"
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
	Name       string
	Nodes      []string            // processing node names (triggers excluded), in order of first appearance
	Edges      []Edge              // edges between processing nodes + inferred END edges (no START edges)
	EntryNodes []string            // processing nodes with no incoming edges from other processing nodes
	Triggers   []string            // trigger node names in order of first appearance
	TriggerMap map[string][]string // trigger name → processing entry node names it connects to
}

// HasTriggers returns true if the workflow has any trigger nodes.
func (rw *ResolvedWorkflow) HasTriggers() bool {
	return len(rw.Triggers) > 0
}

// Predecessors returns the list of predecessor processing node names for the
// given node. Entry nodes (no incoming edges) return an empty slice.
func (rw *ResolvedWorkflow) Predecessors(node string) []string {
	var preds []string
	for _, e := range rw.Edges {
		if e.To == node && e.From != NodeSTART && e.From != NodeEND {
			preds = append(preds, e.From)
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
// Pass nil if no trigger classification is needed (all nodes are processing nodes).
//
// Triggers are separated from processing nodes: they appear in Triggers/TriggerMap
// but not in Nodes/Edges. Processing entry nodes get no implicit START edges —
// the caller (codegen) handles START connections via a router function.
// Implicit END edges are added for processing nodes with no outgoing edges.
func Resolve(block *ast.BlockStatement, isTrigger func(string) bool) ResolvedWorkflow {
	if isTrigger == nil {
		isTrigger = func(string) bool { return false }
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
			// Standalone identifier (no arrows) — treat as a single node.
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

	// Separate triggers from processing nodes (preserving insertion order).
	procGraph := graph.New[string]()
	for _, name := range allGraph.Nodes() {
		if !triggers[name] {
			rw.Nodes = append(rw.Nodes, name)
			procGraph.AddNode(name)
		}
	}

	// Separate trigger edges from processing edges and build TriggerMap.
	for _, e := range allGraph.Edges() {
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
	for _, e := range procGraph.Edges() {
		rw.Edges = append(rw.Edges, Edge{From: e.From, To: e.To})
	}
	for _, leaf := range procGraph.LeafNodes() {
		rw.Edges = append(rw.Edges, Edge{From: leaf, To: NodeEND})
	}

	// Entry nodes: processing nodes with no incoming edges.
	rw.EntryNodes = procGraph.EntryNodes()

	return rw
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
		var indices []string
		for _, idx := range e.Indices {
			indices = append(indices, ExprToNodeName(idx))
		}
		return ExprToNodeName(e.Object) + "[" + strings.Join(indices, ", ") + "]"
	}
	// TODO: Return a unique string.
	return ""
}

// Package workflow provides graph resolution utilities for Orca workflow blocks.
// It extracts edges and node names from AST expressions, shared by both the
// analyzer (validation) and codegen (code generation) stages.
package workflow

import (
	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/graph"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
)

const (
	// Graph terminal node names. These are virtual nodes representing the
	// start and end of a workflow graph — not user-defined blocks.
	NodeSTART = "START"
	NodeEND   = "END"

	// BlockKindBranch is the block kind for branch nodes.
	BlockKindBranch = "branch"

	// Branch schema field names (see compiler/types/bootstrap.orca). If the
	// schema renames these fields, update here too. Exported because the
	// analyzer also looks up these fields when validating route values.
	BranchFieldTransform = "transform"
	BranchFieldRoute     = "route"

	// BranchRouteKeyDefault is the route key used as the fallback target
	// when a branch's transform produces a key not in the explicit route
	// map. Codegen auto-injects {BranchRouteKeyDefault: END} if the user
	// did not provide a "default" entry, so LangGraph never receives an
	// unknown key at runtime.
	BranchRouteKeyDefault = "default"
)

// Edge represents a single directed edge between two named nodes
// in a workflow graph, extracted from arrow expressions.
type Edge struct {
	From string
	To   string
}

// BranchRoute represents a single route in a branch's route table. The
// key is the expression the user wrote ("foo", 42, true, etc.) — kept as
// an ast.Expression so codegen can render it via exprToSource. EntryNodes
// holds the conditional edge target(s): the head node(s) of whatever the
// user wrote on the right side of the arrow.
type BranchRoute struct {
	Key        ast.Expression // route key expression (string | number | bool literal)
	EntryNodes []string       // conditional edge targets — typically one node, the head of the route value
}

// Branch represents a conditional routing point in a workflow graph. The
// branch is itself a processing node — its Name appears in rw.Nodes and gets
// a normal `add_node` call. The transform produces a route key from the
// branch's input (the predecessor's output), and the route table maps keys
// to target nodes via add_conditional_edges.
type Branch struct {
	Name      string         // synthetic name for inline branches, block name for named
	Transform ast.Expression // optional transform lambda applied to input (nil → identity)
	Routes    []BranchRoute  // route table entries (key → target node)
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

// resolveCtx holds the mutable state for a single Resolve call. The walker
// (walkExpr) recurses through workflow expressions and accumulates nodes,
// edges, triggers, and branches into this context. All graph elements —
// including branches — live in a single allGraph; branches are first-class
// processing nodes alongside agents and tools.
type resolveCtx struct {
	symtab           *types.SymbolTable   // symbol table
	allGraph         *graph.Graph[string] // all nodes and edges discovered so far
	triggers         map[string]bool      // set of trigger node names
	triggerOrder     []string             // triggers in first-appearance order
	branches         []Branch             // branches in first-appearance (pre-order) order
	branchSeen       map[string]bool      // branches already extracted (cycle / dedup guard)
	conditionalEdges map[Edge]bool        // edges from a branch to a route entry (emitted via add_conditional_edges, not add_edge)
	isTrigger        func(string) bool
	getBranchBody    func(string) *ast.BlockBody
}

// WalkResult is the return value of walkExpr. Head is the leftmost node of
// the walked sub-expression (used by the caller as a route entry / conditional
// target); Tail is the rightmost node (used by the caller to wire arrow
// chains: `leftTail -> rightHead`). For a single node, Head == Tail. For an
// arrow chain `A -> B -> C`, Head = "A" and Tail = "C". An empty WalkResult
// (both fields "") means the expression did not resolve to a workflow node.
type WalkResult struct {
	Head string
	Tail string
}

// HasTriggers returns true if the workflow has any trigger nodes.
func (rw *ResolvedWorkflow) HasTriggers() bool {
	return len(rw.Triggers) > 0
}

// Predecessors returns the predecessor processing node names of the given
// node, with duplicates removed and first-occurrence order preserved. Entry
// nodes (no incoming edges) return an empty slice.
//
// Branch → route-entry edges are excluded from rw.Edges (codegen emits them
// via add_conditional_edges, not add_edge), so for route entry nodes the
// branch is added back here from rw.Branches. The branch's node function
// stores its input passthrough under its own name, so route targets gather
// from the branch normally — making the branch the correct predecessor.
func (rw *ResolvedWorkflow) Predecessors(node string) []string {
	var preds []string
	seen := make(map[string]bool)
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			preds = append(preds, name)
		}
	}
	for _, e := range rw.Edges {
		if e.To == node && e.From != NodeSTART && e.From != NodeEND {
			add(e.From)
		}
	}
	// If this node is a route entry of any branch, the branch itself is
	// its predecessor (the branch node stores its input as a passthrough).
	for _, branch := range rw.Branches {
		for _, route := range branch.Routes {
			for _, entry := range route.EntryNodes {
				if entry == node {
					add(branch.Name)
					break
				}
			}
		}
	}
	return preds
}

// newResolveCtx creates a resolveCtx with empty state. Nil predicates are
// replaced with no-op defaults so callers can omit them for simple cases.
func newResolveCtx(
	isTrigger func(string) bool,
	getBranchBody func(string) *ast.BlockBody,
	symtab *types.SymbolTable,
) *resolveCtx {
	if isTrigger == nil {
		isTrigger = func(string) bool { return false }
	}
	if getBranchBody == nil {
		getBranchBody = func(string) *ast.BlockBody { return nil }
	}
	return &resolveCtx{
		symtab:           symtab,
		allGraph:         graph.New[string](),
		triggers:         make(map[string]bool),
		branchSeen:       make(map[string]bool),
		conditionalEdges: make(map[Edge]bool),
		isTrigger:        isTrigger,
		getBranchBody:    getBranchBody,
	}
}

// classifyNode registers a node name in allGraph and, if the isTrigger
// predicate matches, records it as a trigger in insertion order. Idempotent —
// safe to call multiple times for the same name.
func (ctx *resolveCtx) classifyNode(name string) {
	if name == "" {
		return
	}
	if ctx.isTrigger(name) {
		if !ctx.triggers[name] {
			ctx.triggers[name] = true
			ctx.triggerOrder = append(ctx.triggerOrder, name)
		}
	}
	ctx.allGraph.AddNode(name)
}

// FindBranch returns the Branch metadata for a node with the given name, or
// nil if no such branch exists. The returned pointer aliases the entry in
// rw.Branches — callers must not retain it across mutations of rw.Branches.
func (rw *ResolvedWorkflow) FindBranch(name string) *Branch {
	for i := range rw.Branches {
		if rw.Branches[i].Name == name {
			return &rw.Branches[i]
		}
	}
	return nil
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

// Resolve extracts nodes, edges, and branches from a workflow block by
// recursively walking its expressions with walkExpr. The isTrigger predicate
// identifies trigger blocks (cron, webhook); the getBranchBody predicate
// looks up named branch blocks from the symbol table. Pass nil for either
// if no classification is needed.
//
// Branches are first-class processing nodes: they appear in rw.Nodes
// alongside agents and tools, and codegen wires their conditional routing
// via add_conditional_edges from the branch node itself.
//
// Triggers are kept separate from processing nodes — they appear in
// Triggers and TriggerMap but not in Nodes or Edges. Implicit END edges
// are added for leaf processing nodes.
func Resolve(
	block *ast.BlockStatement,
	isTrigger func(string) bool,
	getBranchBody func(string) *ast.BlockBody,
	symtab *types.SymbolTable,
) ResolvedWorkflow {
	ctx := newResolveCtx(isTrigger, getBranchBody, symtab)

	rw := ResolvedWorkflow{
		Name:       block.Name,
		TriggerMap: make(map[string][]string),
	}

	// Walk every top-level workflow expression. The walker accumulates
	// nodes, edges, triggers, and branches into ctx.
	for _, expr := range block.Expressions {
		walkExpr(expr, ctx)
	}

	rw.Triggers = ctx.triggerOrder
	rw.Branches = ctx.branches

	// Build procGraph: every non-trigger node from allGraph (branches stay).
	procGraph := graph.New[string]()
	for _, name := range ctx.allGraph.Nodes() {
		if ctx.triggers[name] {
			continue
		}
		rw.Nodes = append(rw.Nodes, name)
		procGraph.AddNode(name)
	}

	// Process edges from allGraph:
	//   - trigger-source edges → TriggerMap
	//   - everything else → procGraph (for leaf detection)
	// Conditional edges (branch → route entry) stay in procGraph so the
	// branch isn't mistakenly treated as a leaf, but they're filtered out
	// of rw.Edges below (codegen emits them via add_conditional_edges).
	//
	// Edges with a trigger destination are dropped entirely. The analyzer
	// rejects this pattern (triggers can only be sources), but we guard
	// here defensively so a malformed AST can't pollute procGraph by
	// re-introducing a trigger as a node via implicit edge insertion.
	for _, e := range ctx.allGraph.Edges() {
		if ctx.triggers[e.To] {
			continue
		}
		if ctx.triggers[e.From] {
			rw.TriggerMap[e.From] = append(rw.TriggerMap[e.From], e.To)
			continue
		}
		procGraph.AddEdge(e.From, e.To)
	}

	// Standalone triggers (not connected via arrows) implicitly connect to
	// all processing nodes (which become their entry targets).
	for _, trig := range rw.Triggers {
		if _, mapped := rw.TriggerMap[trig]; !mapped && len(rw.Nodes) > 0 {
			rw.TriggerMap[trig] = append([]string{}, rw.Nodes...)
		}
	}

	// Materialise rw.Edges from procGraph, skipping conditional branch→entry
	// edges (codegen handles those via add_conditional_edges).
	for _, e := range procGraph.Edges() {
		edge := Edge{From: e.From, To: e.To}
		if ctx.conditionalEdges[edge] {
			continue
		}
		rw.Edges = append(rw.Edges, edge)
	}

	// Infer END edges for leaf processing nodes (no outgoing edges in
	// procGraph, conditional or otherwise). Branches with routes have
	// outgoing conditional edges, so they are not leaves.
	for _, leaf := range procGraph.LeafNodes() {
		rw.Edges = append(rw.Edges, Edge{From: leaf, To: NodeEND})
	}

	// Entry nodes: processing nodes (including branches) with no incoming
	// edges in procGraph.
	rw.EntryNodes = procGraph.EntryNodes()

	return rw
}

// walkExpr walks a workflow expression, accumulating nodes, edges, and
// branches into ctx. Returns a WalkResult containing the leftmost (Head) and
// rightmost (Tail) nodes of the walked expression — see WalkResult docs.
//
// The logic splits into two cases:
//
//   - Arrow chain (A -> B): recurse on both sides, wire an edge from the
//     left's tail to the right's head, return a result spanning both.
//   - Leaf (anything ExprToNodeName can name): classify it, and if it's a
//     branch, extract its transform and routes — which recursively walks
//     the route values, so nested branches fall out automatically.
func walkExpr(expr ast.Expression, ctx *resolveCtx) WalkResult {
	if expr == nil {
		return WalkResult{}
	}

	// Arrow chains recurse on both sides and wire a single edge between them.
	if bin, ok := expr.(*ast.BinaryExpression); ok && bin.Operator.Type == token.ARROW {
		leftRes := walkExpr(bin.Left, ctx)
		rightRes := walkExpr(bin.Right, ctx)
		if leftRes.Tail != "" && rightRes.Head != "" {
			ctx.allGraph.AddEdge(leftRes.Tail, rightRes.Head)
		}
		return WalkResult{Head: leftRes.Head, Tail: rightRes.Tail}
	}

	// If the expression is an orca chain (expr = foo and foo = a -> b).
	if blockBody := types.ExprToBlockBody(expr, ctx.symtab); blockBody != nil {
		if blockBody.Kind == types.AnnotationWorkflowChain {
			left := types.FindAssignment(blockBody, "left")
			right := types.FindAssignment(blockBody, "right")
			if left != nil && right != nil {
				leftRes := walkExpr(left.Value, ctx)
				rightRes := walkExpr(right.Value, ctx)
				if leftRes.Tail != "" && rightRes.Head != "" {
					ctx.allGraph.AddEdge(leftRes.Tail, rightRes.Head)
				}
				return WalkResult{Head: leftRes.Head, Tail: rightRes.Tail}
			}
		}
	}

	// Leaf: resolve the expression to a node name and classify it. If it's
	// a branch (inline or named), extract its transform and routes.
	name := ExprToNodeName(expr, ctx.symtab)
	if name == "" {
		return WalkResult{}
	}
	ctx.classifyNode(name)
	if body := branchBodyFor(expr, ctx); body != nil {
		extractBranch(name, body, ctx)
	}
	return WalkResult{Head: name, Tail: name}
}

// branchBodyFor returns the BlockBody of a branch leaf expression, or nil if
// the expression is not a branch. Handles both inline `branch { ... }` blocks
// and named branch references resolved via ctx.getBranchBody.
func branchBodyFor(expr ast.Expression, ctx *resolveCtx) *ast.BlockBody {
	switch e := expr.(type) {
	case *ast.Identifier:
		return ctx.getBranchBody(e.Value)
	case *ast.BlockExpression:
		if e.Kind == BlockKindBranch {
			return &e.BlockBody
		}
	}
	return nil
}

// extractBranch extracts a branch's transform and routes, recursively walking
// each route value. The branch is appended to ctx.branches in pre-order (the
// outer branch appears before any nested inner branches it contains). Cycle
// guarded by ctx.branchSeen so a branch referenced multiple times is only
// extracted once.
//
// For each route, the head of the walked route value becomes the conditional
// target. An edge from branch_name → route_head is added to allGraph and
// marked in conditionalEdges so codegen knows to emit it via
// add_conditional_edges instead of add_edge.
func extractBranch(name string, body *ast.BlockBody, ctx *resolveCtx) {
	if ctx.branchSeen[name] {
		return
	}
	ctx.branchSeen[name] = true

	branch := Branch{Name: name}
	if t, ok := body.GetFieldExpression(BranchFieldTransform); ok {
		branch.Transform = t
	}

	// Append the branch BEFORE walking routes so nested inner branches appear
	// after this one in ctx.branches (pre-order, matches source visit order).
	// We mutate ctx.branches[idx].Routes after walking each route.
	ctx.branches = append(ctx.branches, branch)
	idx := len(ctx.branches) - 1

	routeExpr, ok := body.GetFieldExpression(BranchFieldRoute)
	if !ok {
		return
	}
	mapLit, ok := routeExpr.(*ast.MapLiteral)
	if !ok {
		return
	}

	for _, entry := range mapLit.Entries {
		routeRes := walkExpr(entry.Value, ctx)
		route := BranchRoute{Key: entry.Key}
		if routeRes.Head != "" {
			route.EntryNodes = []string{routeRes.Head}
			ctx.conditionalEdges[Edge{From: name, To: routeRes.Head}] = true
			ctx.allGraph.AddEdge(name, routeRes.Head)
		}
		// ctx.branches[idx] is the correct slot even if a recursive walkExpr
		// caused ctx.branches to grow — slice indexing handles realloc.
		ctx.branches[idx].Routes = append(ctx.branches[idx].Routes, route)
	}
}

// EdgesFromExpr walks a (possibly chained) arrow expression and returns
// the list of individual edges. For example, A -> B -> C yields:
// [{A, B}, {B, C}]. Non-arrow expressions return nil.
func EdgesFromExpr(expr ast.Expression, symtab *types.SymbolTable) []Edge {
	bin, ok := expr.(*ast.BinaryExpression)
	if !ok || bin.Operator.Type != token.ARROW {
		return nil
	}

	// The parser builds left-associative trees: ((A -> B) -> C).
	// Recursively flatten the left side, then connect its last node to the right.
	leftEdges := EdgesFromExpr(bin.Left, symtab)

	rightName := ExprToNodeName(bin.Right, symtab)

	if len(leftEdges) > 0 {
		lastTo := leftEdges[len(leftEdges)-1].To
		return append(leftEdges, Edge{From: lastTo, To: rightName})
	}

	leftName := ExprToNodeName(bin.Left, symtab)
	return []Edge{{From: leftName, To: rightName}}
}

// ExprToNodeName returns the workflow node name for an expression.
// For inline BlockExpression nodes, generates a synthetic name like "__branch_0".
// Returns empty string for unrecognized expression types.
func ExprToNodeName(expr ast.Expression, symtab *types.SymbolTable) string {
	if symtab != nil {
		if block := types.ExprToBlockBody(expr, symtab); block != nil {
			return block.Name
		}
	}

	// A fallback method when symbol table is null (Only in tests)
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Value
	case *ast.BlockExpression:
		return e.Name
	}
	return ""
}

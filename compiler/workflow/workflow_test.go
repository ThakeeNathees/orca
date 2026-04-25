package workflow

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
)

// buildAgentsSymtab constructs a symbol table mimicking:
//
//	let agents {
//	    researcher = agent { ... }
//	    writer     = agent { ... }
//	}
//
// so that MemberAccess expressions like `agents.researcher` resolve through
// types.ExprToBlockBody to the inner BlockExpression, whose BlockBody.Name
// is what ExprToNodeName returns as the workflow node name.
func buildAgentsSymtab() *types.SymbolTable {
	researcher := &ast.BlockExpression{
		BlockBody: ast.BlockBody{Name: "agents.researcher", Kind: "agent"},
	}
	writer := &ast.BlockExpression{
		BlockBody: ast.BlockBody{Name: "agents.writer", Kind: "agent"},
	}
	agentsBody := &ast.BlockBody{
		Name: "agents",
		Kind: "let",
		Assignments: []*ast.Assignment{
			{Name: "researcher", Value: researcher},
			{Name: "writer", Value: writer},
		},
	}
	schema := &types.BlockSchema{BlockName: "agents", Ast: agentsBody}
	symtab := types.NewSymbolTable()
	symtab.Define("agents", types.NewBlockRefType("agents", schema), token.Token{})
	return &symtab
}

// ident creates an Identifier expression for testing.
func ident(name string) *ast.Identifier {
	return &ast.Identifier{Value: name}
}

// arrow creates a BinaryExpression with the ARROW operator for testing.
func arrow(left, right ast.Expression) *ast.BinaryExpression {
	return &ast.BinaryExpression{
		Left:     left,
		Operator: token.Token{Type: token.ARROW},
		Right:    right,
	}
}

func TestEdgesFromExpr(t *testing.T) {
	tests := []struct {
		name     string
		expr     ast.Expression
		expected []Edge
	}{
		{
			"simple A -> B",
			arrow(ident("A"), ident("B")),
			[]Edge{{From: "A", To: "B"}},
		},
		{
			"chain A -> B -> C",
			arrow(arrow(ident("A"), ident("B")), ident("C")),
			[]Edge{{From: "A", To: "B"}, {From: "B", To: "C"}},
		},
		{
			"non-arrow expression returns nil",
			ident("A"),
			nil,
		},
		{
			"long chain A -> B -> C -> D",
			arrow(arrow(arrow(ident("A"), ident("B")), ident("C")), ident("D")),
			[]Edge{
				{From: "A", To: "B"},
				{From: "B", To: "C"},
				{From: "C", To: "D"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EdgesFromExpr(tt.expr, nil)
			if len(got) != len(tt.expected) {
				t.Fatalf("EdgesFromExpr() = %v, want %v", got, tt.expected)
			}
			for i, e := range got {
				if e != tt.expected[i] {
					t.Errorf("edge[%d] = %v, want %v", i, e, tt.expected[i])
				}
			}
		})
	}
}

func TestResolve(t *testing.T) {
	// Helper to build a workflow block with expressions.
	makeBlock := func(name string, exprs ...ast.Expression) *ast.BlockStatement {
		return &ast.BlockStatement{
			BlockBody: ast.BlockBody{Name: name, Expressions: exprs},
		}
	}

	tests := []struct {
		name             string
		block            *ast.BlockStatement
		isTrigger        func(string) bool
		expectedNodes    []string
		expectedTriggers []string
		expectedEntries  []string
		expectedTrigMap  map[string][]string
	}{
		{
			"no triggers single chain",
			makeBlock("pipeline", arrow(ident("A"), ident("B"))),
			nil,
			[]string{"A", "B"},
			nil,
			[]string{"A"},
			map[string][]string{},
		},
		{
			"trigger strips from nodes",
			makeBlock("pipeline",
				arrow(arrow(ident("daily"), ident("A")), ident("B")),
			),
			func(name string) bool { return name == "daily" },
			[]string{"A", "B"},
			[]string{"daily"},
			[]string{"A"},
			map[string][]string{"daily": {"A"}},
		},
		{
			"multiple triggers different entries",
			makeBlock("pipeline",
				arrow(arrow(ident("daily"), ident("A")), ident("C")),
				arrow(arrow(ident("hooks"), ident("B")), ident("C")),
			),
			func(name string) bool { return name == "daily" || name == "hooks" },
			[]string{"A", "C", "B"},
			[]string{"daily", "hooks"},
			[]string{"A", "B"},
			map[string][]string{"daily": {"A"}, "hooks": {"B"}},
		},
		{
			"single standalone node",
			makeBlock("pipeline", ident("A")),
			nil,
			[]string{"A"},
			nil,
			[]string{"A"},
			map[string][]string{},
		},
		{
			"standalone node with arrows",
			makeBlock("pipeline", ident("A"), arrow(ident("B"), ident("C"))),
			nil,
			[]string{"A", "B", "C"},
			nil,
			[]string{"A", "B"},
			map[string][]string{},
		},
		{
			"standalone trigger node",
			makeBlock("pipeline", ident("daily"), ident("A")),
			func(name string) bool { return name == "daily" },
			[]string{"A"},
			[]string{"daily"},
			[]string{"A"},
			map[string][]string{"daily": {"A"}},
		},
		{
			"fan-out trigger",
			makeBlock("pipeline",
				arrow(ident("daily"), ident("A")),
				arrow(ident("daily"), ident("B")),
				arrow(ident("A"), ident("C")),
				arrow(ident("B"), ident("C")),
			),
			func(name string) bool { return name == "daily" },
			[]string{"A", "B", "C"},
			[]string{"daily"},
			[]string{"A", "B"},
			map[string][]string{"daily": {"A", "B"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := Resolve(tt.block, tt.isTrigger, nil, nil)

			// Check nodes.
			if len(rw.Nodes) != len(tt.expectedNodes) {
				t.Fatalf("Nodes = %v, want %v", rw.Nodes, tt.expectedNodes)
			}
			for i, n := range rw.Nodes {
				if n != tt.expectedNodes[i] {
					t.Errorf("Nodes[%d] = %q, want %q", i, n, tt.expectedNodes[i])
				}
			}

			// Check triggers.
			if len(rw.Triggers) != len(tt.expectedTriggers) {
				t.Fatalf("Triggers = %v, want %v", rw.Triggers, tt.expectedTriggers)
			}
			for i, tr := range rw.Triggers {
				if tr != tt.expectedTriggers[i] {
					t.Errorf("Triggers[%d] = %q, want %q", i, tr, tt.expectedTriggers[i])
				}
			}

			// Check entry nodes.
			if len(rw.EntryNodes) != len(tt.expectedEntries) {
				t.Fatalf("EntryNodes = %v, want %v", rw.EntryNodes, tt.expectedEntries)
			}
			for i, en := range rw.EntryNodes {
				if en != tt.expectedEntries[i] {
					t.Errorf("EntryNodes[%d] = %q, want %q", i, en, tt.expectedEntries[i])
				}
			}

			// Check trigger map.
			for trig, expectedTargets := range tt.expectedTrigMap {
				gotTargets := rw.TriggerMap[trig]
				if len(gotTargets) != len(expectedTargets) {
					t.Errorf("TriggerMap[%q] = %v, want %v", trig, gotTargets, expectedTargets)
					continue
				}
				for i, target := range gotTargets {
					if target != expectedTargets[i] {
						t.Errorf("TriggerMap[%q][%d] = %q, want %q", trig, i, target, expectedTargets[i])
					}
				}
			}

			// Verify no START edges in the result.
			for _, e := range rw.Edges {
				if e.From == types.NodeSTART {
					t.Errorf("unexpected START edge: %v", e)
				}
			}
		})
	}
}

func TestExprToNodeName(t *testing.T) {
	agentsSymtab := buildAgentsSymtab()

	tests := []struct {
		name     string
		expr     ast.Expression
		symtab   *types.SymbolTable
		expected string
	}{
		{
			"identifier",
			ident("foo"),
			nil,
			"foo",
		},
		{
			// MemberAccess resolves through the symbol table: agents →
			// let block → researcher assignment → inner BlockExpression
			// whose BlockBody.Name is "agents.researcher".
			"member access",
			&ast.MemberAccess{Object: ident("agents"), Member: "researcher"},
			agentsSymtab,
			"agents.researcher",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExprToNodeName(tt.expr, tt.symtab)
			if got != tt.expected {
				t.Errorf("ExprToNodeName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestPredecessors verifies predecessor computation from the resolved workflow.
func TestPredecessors(t *testing.T) {
	// Build a workflow: cron -> A -> B -> D, A -> C -> D (diamond with trigger).
	block := &ast.BlockStatement{
		BlockBody: ast.BlockBody{
			Name: "pipeline",
			Kind: "workflow",
			Expressions: []ast.Expression{
				arrow(arrow(ident("cron"), ident("A")), ident("B")),
				arrow(ident("A"), ident("C")),
				arrow(ident("B"), ident("D")),
				arrow(ident("C"), ident("D")),
			},
		},
	}

	isTrigger := func(name string) bool { return name == "cron" }
	rw := Resolve(block, isTrigger, nil, nil)

	tests := []struct {
		name     string
		node     string
		expected []string
	}{
		{"entry node has no predecessors", "A", nil},
		{"single predecessor", "B", []string{"A"}},
		{"single predecessor C", "C", []string{"A"}},
		{"fan-in node has multiple predecessors", "D", []string{"B", "C"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rw.Predecessors(tt.node)
			if len(got) != len(tt.expected) {
				t.Fatalf("Predecessors(%q) = %v, want %v", tt.node, got, tt.expected)
			}
			for i, g := range got {
				if g != tt.expected[i] {
					t.Errorf("Predecessors(%q)[%d] = %q, want %q", tt.node, i, g, tt.expected[i])
				}
			}
		})
	}
}

// TestPredecessorsNoTrigger verifies predecessors when there's no trigger.
func TestPredecessorsNoTrigger(t *testing.T) {
	block := &ast.BlockStatement{
		BlockBody: ast.BlockBody{
			Name: "simple",
			Kind: "workflow",
			Expressions: []ast.Expression{
				arrow(ident("A"), ident("B")),
			},
		},
	}

	rw := Resolve(block, nil, nil, nil)

	got := rw.Predecessors("A")
	if len(got) != 0 {
		t.Errorf("Predecessors(A) = %v, want []", got)
	}

	got = rw.Predecessors("B")
	if len(got) != 1 || got[0] != "A" {
		t.Errorf("Predecessors(B) = %v, want [A]", got)
	}
}

// TestWalkExpr verifies the unified walker correctly handles non-branch
// expression shapes: identifiers, member access, subscriptions, and arrow
// chains. WalkResult.Head is the leftmost node, WalkResult.Tail is the
// rightmost.
func TestWalkExpr(t *testing.T) {
	agentsSymtab := buildAgentsSymtab()

	tests := []struct {
		name          string
		expr          ast.Expression
		symtab        *types.SymbolTable
		expectedHead  string
		expectedTail  string
		expectedNodes []string
		expectedEdges []Edge
	}{
		{
			"single identifier",
			ident("A"),
			nil,
			"A", "A",
			[]string{"A"},
			nil,
		},
		{
			"simple arrow A -> B",
			arrow(ident("A"), ident("B")),
			nil,
			"A", "B",
			[]string{"A", "B"},
			[]Edge{{From: "A", To: "B"}},
		},
		{
			"chain A -> B -> C",
			arrow(arrow(ident("A"), ident("B")), ident("C")),
			nil,
			"A", "C",
			[]string{"A", "B", "C"},
			[]Edge{{From: "A", To: "B"}, {From: "B", To: "C"}},
		},
		{
			"long chain A -> B -> C -> D",
			arrow(arrow(arrow(ident("A"), ident("B")), ident("C")), ident("D")),
			nil,
			"A", "D",
			[]string{"A", "B", "C", "D"},
			[]Edge{
				{From: "A", To: "B"},
				{From: "B", To: "C"},
				{From: "C", To: "D"},
			},
		},
		{
			"member access node",
			&ast.MemberAccess{Object: ident("agents"), Member: "researcher"},
			agentsSymtab,
			"agents.researcher", "agents.researcher",
			[]string{"agents.researcher"},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newResolveCtx(nil, nil, tt.symtab)
			res := walkExpr(tt.expr, ctx)
			if res.Head != tt.expectedHead {
				t.Errorf("Head = %q, want %q", res.Head, tt.expectedHead)
			}
			if res.Tail != tt.expectedTail {
				t.Errorf("Tail = %q, want %q", res.Tail, tt.expectedTail)
			}

			gotNodes := ctx.allGraph.Nodes()
			if len(gotNodes) != len(tt.expectedNodes) {
				t.Fatalf("nodes = %v, want %v", gotNodes, tt.expectedNodes)
			}
			for i, n := range gotNodes {
				if n != tt.expectedNodes[i] {
					t.Errorf("nodes[%d] = %q, want %q", i, n, tt.expectedNodes[i])
				}
			}

			gotEdges := ctx.allGraph.Edges()
			if len(gotEdges) != len(tt.expectedEdges) {
				t.Fatalf("edges = %v, want %v", gotEdges, tt.expectedEdges)
			}
			for i, e := range gotEdges {
				ee := Edge{From: e.From, To: e.To}
				if ee != tt.expectedEdges[i] {
					t.Errorf("edges[%d] = %v, want %v", i, ee, tt.expectedEdges[i])
				}
			}
		})
	}
}

// TestWalkExprTriggers verifies the walker classifies trigger nodes correctly
// and tracks them in insertion order separately from regular nodes.
func TestWalkExprTriggers(t *testing.T) {
	// cron -> A -> B, where cron is a trigger.
	expr := arrow(arrow(ident("cron"), ident("A")), ident("B"))
	ctx := newResolveCtx(func(n string) bool { return n == "cron" }, nil, nil)

	res := walkExpr(expr, ctx)
	if res.Tail != "B" {
		t.Errorf("Tail = %q, want B", res.Tail)
	}
	if res.Head != "cron" {
		t.Errorf("Head = %q, want cron", res.Head)
	}
	if !ctx.triggers["cron"] {
		t.Errorf("expected cron to be marked as trigger")
	}
	if len(ctx.triggerOrder) != 1 || ctx.triggerOrder[0] != "cron" {
		t.Errorf("triggerOrder = %v, want [cron]", ctx.triggerOrder)
	}
	// Triggers are still in allGraph — separation happens in Resolve, not the walker.
	if !ctx.allGraph.HasNode("cron") {
		t.Errorf("expected cron in allGraph")
	}
	if !ctx.allGraph.HasEdge("cron", "A") {
		t.Errorf("expected edge cron -> A in allGraph")
	}
}

// TestWalkExprBranchInline verifies the walker treats an inline branch as
// a real node: the branch is added to allGraph, its routes are extracted
// into ctx.branches, and conditional edges from branch → route entries are
// recorded.
func TestWalkExprBranchInline(t *testing.T) {
	// A -> branch { route = { "x": B, "y": C } }
	br := inlineBranch("br0", nil,
		ast.MapEntry{Key: strKey("x"), Value: ident("B")},
		ast.MapEntry{Key: strKey("y"), Value: ident("C")},
	)
	expr := arrow(ident("A"), br)

	ctx := newResolveCtx(nil, nil, nil)
	res := walkExpr(expr, ctx)

	// Chain head/tail.
	if res.Head != "A" {
		t.Errorf("Head = %q, want A", res.Head)
	}
	if res.Tail != "br0" {
		t.Errorf("Tail = %q, want br0", res.Tail)
	}

	// Nodes in allGraph: A, br0 (the branch IS a node), B, C.
	expectedNodes := []string{"A", "br0", "B", "C"}
	if len(ctx.allGraph.Nodes()) != len(expectedNodes) {
		t.Fatalf("Nodes = %v, want %v", ctx.allGraph.Nodes(), expectedNodes)
	}
	for i, n := range ctx.allGraph.Nodes() {
		if n != expectedNodes[i] {
			t.Errorf("Nodes[%d] = %q, want %q", i, n, expectedNodes[i])
		}
	}

	// Edges in allGraph: A→br0 (regular), br0→B (conditional), br0→C (conditional).
	if !ctx.allGraph.HasEdge("A", "br0") {
		t.Errorf("missing edge A -> br0")
	}
	if !ctx.allGraph.HasEdge("br0", "B") {
		t.Errorf("missing edge br0 -> B")
	}
	if !ctx.allGraph.HasEdge("br0", "C") {
		t.Errorf("missing edge br0 -> C")
	}
	// br0→B and br0→C should be marked conditional, A→br0 should NOT.
	if !ctx.conditionalEdges[Edge{From: "br0", To: "B"}] {
		t.Errorf("expected br0 -> B to be conditional")
	}
	if !ctx.conditionalEdges[Edge{From: "br0", To: "C"}] {
		t.Errorf("expected br0 -> C to be conditional")
	}
	if ctx.conditionalEdges[Edge{From: "A", To: "br0"}] {
		t.Errorf("A -> br0 should be a regular edge, not conditional")
	}

	// Branch metadata.
	if len(ctx.branches) != 1 {
		t.Fatalf("branches = %d, want 1", len(ctx.branches))
	}
	br0 := ctx.branches[0]
	if br0.Name != "br0" {
		t.Errorf("branch.Name = %q, want br0", br0.Name)
	}
	if len(br0.Routes) != 2 {
		t.Fatalf("routes = %d, want 2", len(br0.Routes))
	}
	if len(br0.Routes[0].EntryNodes) != 1 || br0.Routes[0].EntryNodes[0] != "B" {
		t.Errorf("Routes[0].EntryNodes = %v, want [B]", br0.Routes[0].EntryNodes)
	}
	if len(br0.Routes[1].EntryNodes) != 1 || br0.Routes[1].EntryNodes[0] != "C" {
		t.Errorf("Routes[1].EntryNodes = %v, want [C]", br0.Routes[1].EntryNodes)
	}
}

// TestWalkExprBranchChainRoute verifies the walker handles route values that
// are arrow chains (e.g. `route = { "x": B -> C }`). The route's entry is the
// leftmost node (B); the chain B→C is a regular edge in allGraph.
func TestWalkExprBranchChainRoute(t *testing.T) {
	// A -> branch { route = { "x": B -> C, "y": D } }
	br := inlineBranch("br0", nil,
		ast.MapEntry{Key: strKey("x"), Value: arrow(ident("B"), ident("C"))},
		ast.MapEntry{Key: strKey("y"), Value: ident("D")},
	)
	expr := arrow(ident("A"), br)

	ctx := newResolveCtx(nil, nil, nil)
	walkExpr(expr, ctx)

	// Conditional edge: br0 -> B (the head of the chain), not br0 -> C.
	if !ctx.conditionalEdges[Edge{From: "br0", To: "B"}] {
		t.Errorf("expected conditional edge br0 -> B")
	}
	if ctx.conditionalEdges[Edge{From: "br0", To: "C"}] {
		t.Errorf("br0 -> C should NOT be conditional (B->C is the chain)")
	}

	// Regular edge: B -> C.
	if !ctx.allGraph.HasEdge("B", "C") {
		t.Errorf("missing regular edge B -> C")
	}
	if ctx.conditionalEdges[Edge{From: "B", To: "C"}] {
		t.Errorf("B -> C should NOT be conditional")
	}

	// Branch routes.
	if len(ctx.branches) != 1 {
		t.Fatalf("branches = %d, want 1", len(ctx.branches))
	}
	if ctx.branches[0].Routes[0].EntryNodes[0] != "B" {
		t.Errorf("Route 0 entry = %q, want B", ctx.branches[0].Routes[0].EntryNodes[0])
	}
}

// TestWalkExprBranchNested verifies the walker handles nested branches —
// a branch whose route value is itself a branch. Both branches are recorded
// in pre-order (outer first), and conditional edges chain correctly.
func TestWalkExprBranchNested(t *testing.T) {
	// A -> outer { route = { "x": inner { route = { "y": C } } } }
	inner := inlineBranch("inner", nil,
		ast.MapEntry{Key: strKey("y"), Value: ident("C")},
	)
	outer := inlineBranch("outer", nil,
		ast.MapEntry{Key: strKey("x"), Value: inner},
	)
	expr := arrow(ident("A"), outer)

	ctx := newResolveCtx(nil, nil, nil)
	walkExpr(expr, ctx)

	// Both branches recorded, outer first (pre-order).
	if len(ctx.branches) != 2 {
		t.Fatalf("branches = %d, want 2", len(ctx.branches))
	}
	if ctx.branches[0].Name != "outer" {
		t.Errorf("branches[0].Name = %q, want outer", ctx.branches[0].Name)
	}
	if ctx.branches[1].Name != "inner" {
		t.Errorf("branches[1].Name = %q, want inner", ctx.branches[1].Name)
	}

	// Outer's route "x" → inner (the inner branch is a real node).
	if ctx.branches[0].Routes[0].EntryNodes[0] != "inner" {
		t.Errorf("outer route 0 entry = %q, want inner", ctx.branches[0].Routes[0].EntryNodes[0])
	}
	// Inner's route "y" → C.
	if ctx.branches[1].Routes[0].EntryNodes[0] != "C" {
		t.Errorf("inner route 0 entry = %q, want C", ctx.branches[1].Routes[0].EntryNodes[0])
	}

	// Conditional edges: outer→inner, inner→C.
	if !ctx.conditionalEdges[Edge{From: "outer", To: "inner"}] {
		t.Errorf("expected conditional edge outer -> inner")
	}
	if !ctx.conditionalEdges[Edge{From: "inner", To: "C"}] {
		t.Errorf("expected conditional edge inner -> C")
	}
	// Regular edge: A → outer.
	if !ctx.allGraph.HasEdge("A", "outer") {
		t.Errorf("missing regular edge A -> outer")
	}
	if ctx.conditionalEdges[Edge{From: "A", To: "outer"}] {
		t.Errorf("A -> outer should be a regular edge")
	}
}

// TestWalkExprBranchNamed verifies the walker resolves named branches via
// the getBranchBody lookup function (mimicking how codegen passes the symbol
// table lookup).
func TestWalkExprBranchNamed(t *testing.T) {
	// A -> my_branch (named branch defined elsewhere)
	branchBody := &ast.BlockBody{
		Kind: types.BlockKindBranch,
		Assignments: []*ast.Assignment{
			{Name: "route", Value: &ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: strKey("x"), Value: ident("B")},
			}}},
		},
	}
	getBranchBody := func(name string) *ast.BlockBody {
		if name == "my_branch" {
			return branchBody
		}
		return nil
	}

	expr := arrow(ident("A"), ident("my_branch"))
	ctx := newResolveCtx(nil, getBranchBody, nil)
	walkExpr(expr, ctx)

	if len(ctx.branches) != 1 {
		t.Fatalf("branches = %d, want 1", len(ctx.branches))
	}
	if ctx.branches[0].Name != "my_branch" {
		t.Errorf("branches[0].Name = %q, want my_branch", ctx.branches[0].Name)
	}
	if !ctx.allGraph.HasEdge("A", "my_branch") {
		t.Errorf("missing edge A -> my_branch")
	}
	if !ctx.conditionalEdges[Edge{From: "my_branch", To: "B"}] {
		t.Errorf("missing conditional edge my_branch -> B")
	}
}

// TestWalkExprBranchCycle verifies the branchSeen guard prevents infinite
// recursion when a branch is referenced multiple times (e.g., via getBranchBody
// returning the same branch from a route value that loops back).
func TestWalkExprBranchCycle(t *testing.T) {
	// my_branch with route "again" -> my_branch (loops back to itself).
	var branchBody *ast.BlockBody
	branchBody = &ast.BlockBody{
		Kind: types.BlockKindBranch,
		Assignments: []*ast.Assignment{
			{Name: "route", Value: &ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: strKey("again"), Value: ident("my_branch")},
				{Key: strKey("done"), Value: ident("B")},
			}}},
		},
	}
	getBranchBody := func(name string) *ast.BlockBody {
		if name == "my_branch" {
			return branchBody
		}
		return nil
	}

	ctx := newResolveCtx(nil, getBranchBody, nil)
	res := walkExpr(ident("my_branch"), ctx)

	// Walker terminated without infinite recursion.
	if res.Head != "my_branch" {
		t.Errorf("Head = %q, want my_branch", res.Head)
	}
	if len(ctx.branches) != 1 {
		t.Fatalf("branches = %d, want 1 (cycle should not duplicate)", len(ctx.branches))
	}
	// The branch's "again" route still wires a conditional edge back to itself.
	if !ctx.conditionalEdges[Edge{From: "my_branch", To: "my_branch"}] {
		t.Errorf("expected self-loop conditional edge my_branch -> my_branch")
	}
}

// TestWalkExprMultipleExpressions verifies the walker accumulates state
// correctly across multiple top-level expressions sharing the same context.
// This mirrors how Resolve will invoke walkExpr in a loop over block.Expressions.
func TestWalkExprMultipleExpressions(t *testing.T) {
	ctx := newResolveCtx(nil, nil, nil)
	walkExpr(arrow(ident("A"), ident("B")), ctx)
	walkExpr(arrow(ident("C"), ident("B")), ctx)

	expectedNodes := []string{"A", "B", "C"}
	gotNodes := ctx.allGraph.Nodes()
	if len(gotNodes) != len(expectedNodes) {
		t.Fatalf("nodes = %v, want %v", gotNodes, expectedNodes)
	}
	for i, n := range gotNodes {
		if n != expectedNodes[i] {
			t.Errorf("nodes[%d] = %q, want %q", i, n, expectedNodes[i])
		}
	}
	if !ctx.allGraph.HasEdge("A", "B") || !ctx.allGraph.HasEdge("C", "B") {
		t.Errorf("missing expected edges in allGraph")
	}
}

// inlineBranch creates an inline branch BlockExpression with a route map for testing.
// The name parameter is used as BlockNameAnon to simulate the type system having
// already registered this block in the symbol table.
func inlineBranch(name string, transform ast.Expression, routes ...ast.MapEntry) *ast.BlockExpression {
	assignments := []*ast.Assignment{}
	if transform != nil {
		assignments = append(assignments, &ast.Assignment{Name: "transform", Value: transform})
	}
	assignments = append(assignments, &ast.Assignment{
		Name:  "route",
		Value: &ast.MapLiteral{Entries: routes},
	})
	return &ast.BlockExpression{
		BlockBody: ast.BlockBody{
			Name:        name,
			Kind:        "branch",
			Assignments: assignments,
		},
	}
}

// branchLookup builds a getBranchBody function from inline branch BlockExpressions.
func branchLookup(branches ...*ast.BlockExpression) func(string) *ast.BlockBody {
	m := make(map[string]*ast.BlockBody, len(branches))
	for _, b := range branches {
		m[b.Name] = &b.BlockBody
	}
	return func(name string) *ast.BlockBody {
		return m[name]
	}
}

// strKey creates a string literal for use as a map key.
func strKey(s string) *ast.StringLiteral {
	return &ast.StringLiteral{Value: s}
}

func TestResolveBranch(t *testing.T) {
	makeBlock := func(name string, exprs ...ast.Expression) *ast.BlockStatement {
		return &ast.BlockStatement{
			BlockBody: ast.BlockBody{Name: name, Expressions: exprs},
		}
	}

	// Build inline branch test cases with shared BlockExpression instances.
	simpleBranch := inlineBranch("br_simple", nil,
		ast.MapEntry{Key: strKey("x"), Value: ident("B")},
		ast.MapEntry{Key: strKey("y"), Value: ident("C")},
	)
	chainBranch := inlineBranch("br_chain", nil,
		ast.MapEntry{Key: strKey("foo"), Value: arrow(ident("B"), ident("C"))},
		ast.MapEntry{Key: strKey("bar"), Value: ident("D")},
	)
	complexBranch := inlineBranch("br_complex", nil,
		ast.MapEntry{Key: strKey("x"), Value: arrow(ident("B"), ident("D"))},
		ast.MapEntry{Key: strKey("y"), Value: arrow(ident("C"), ident("D"))},
	)

	tests := []struct {
		name            string
		block           *ast.BlockStatement
		getBranchBody   func(string) *ast.BlockBody
		expectedNodes   []string
		expectedEdges   []Edge
		expectedEntries []string
		branchCount     int
		branchRoutes    int
	}{
		{
			// br_simple appears in Nodes (branches are real nodes); A →
			// br_simple is a regular edge; B and C are END leaves; the
			// br_simple → B and br_simple → C edges are conditional and live
			// in rw.Branches[0].Routes, not rw.Edges.
			"inline branch with simple routes",
			makeBlock("wf", arrow(ident("A"), simpleBranch)),
			branchLookup(simpleBranch),
			[]string{"A", "br_simple", "B", "C"},
			[]Edge{
				{From: "A", To: "br_simple"},
				{From: "B", To: types.NodeEND},
				{From: "C", To: types.NodeEND},
			},
			[]string{"A"},
			1,
			2,
		},
		{
			// Route value `B -> C` puts B as the conditional target; B → C is
			// a regular edge in rw.Edges; D is its own simple route.
			"inline branch with chain routes",
			makeBlock("wf", arrow(ident("A"), chainBranch)),
			branchLookup(chainBranch),
			[]string{"A", "br_chain", "B", "C", "D"},
			[]Edge{
				{From: "B", To: "C"},
				{From: "A", To: "br_chain"},
				{From: "C", To: types.NodeEND},
				{From: "D", To: types.NodeEND},
			},
			[]string{"A"},
			1,
			2,
		},
		{
			// Named branch referenced from two predecessors. branchSeen guards
			// against double-extraction; both A and B regular-edge into my_branch.
			"named branch with multiple predecessors",
			makeBlock("wf",
				arrow(ident("A"), ident("my_branch")),
				arrow(ident("B"), ident("my_branch")),
			),
			func(name string) *ast.BlockBody {
				if name == "my_branch" {
					return &ast.BlockBody{
						Kind: "branch",
						Assignments: []*ast.Assignment{
							{Name: "route", Value: &ast.MapLiteral{Entries: []ast.MapEntry{
								{Key: strKey("x"), Value: ident("C")},
								{Key: strKey("y"), Value: ident("D")},
							}}},
						},
					}
				}
				return nil
			},
			[]string{"A", "my_branch", "C", "D", "B"},
			[]Edge{
				{From: "A", To: "my_branch"},
				{From: "B", To: "my_branch"},
				{From: "C", To: types.NodeEND},
				{From: "D", To: types.NodeEND},
			},
			[]string{"A", "B"},
			1,
			2,
		},
		{
			// Two route chains that converge at D — D appears once and is a
			// shared END leaf. br_complex → B and br_complex → C are
			// conditional, B → D and C → D are regular.
			"branch with complex subgraph in route",
			makeBlock("wf", arrow(ident("A"), complexBranch)),
			branchLookup(complexBranch),
			[]string{"A", "br_complex", "B", "D", "C"},
			[]Edge{
				{From: "B", To: "D"},
				{From: "C", To: "D"},
				{From: "A", To: "br_complex"},
				{From: "D", To: types.NodeEND},
			},
			[]string{"A"},
			1,
			2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := Resolve(tt.block, nil, tt.getBranchBody, nil)

			// Check nodes.
			if len(rw.Nodes) != len(tt.expectedNodes) {
				t.Fatalf("Nodes = %v, want %v", rw.Nodes, tt.expectedNodes)
			}
			for i, n := range rw.Nodes {
				if n != tt.expectedNodes[i] {
					t.Errorf("Nodes[%d] = %q, want %q", i, n, tt.expectedNodes[i])
				}
			}

			// Check edges.
			if len(rw.Edges) != len(tt.expectedEdges) {
				t.Fatalf("Edges = %v, want %v", rw.Edges, tt.expectedEdges)
			}
			for i, e := range rw.Edges {
				if e != tt.expectedEdges[i] {
					t.Errorf("Edges[%d] = %v, want %v", i, e, tt.expectedEdges[i])
				}
			}

			// Check entry nodes.
			if len(rw.EntryNodes) != len(tt.expectedEntries) {
				t.Fatalf("EntryNodes = %v, want %v", rw.EntryNodes, tt.expectedEntries)
			}
			for i, en := range rw.EntryNodes {
				if en != tt.expectedEntries[i] {
					t.Errorf("EntryNodes[%d] = %q, want %q", i, en, tt.expectedEntries[i])
				}
			}

			// Check branch count.
			if len(rw.Branches) != tt.branchCount {
				t.Fatalf("Branches count = %d, want %d", len(rw.Branches), tt.branchCount)
			}

			// Check total route count.
			totalRoutes := 0
			for _, b := range rw.Branches {
				totalRoutes += len(b.Routes)
			}
			if totalRoutes != tt.branchRoutes {
				t.Errorf("total routes = %d, want %d", totalRoutes, tt.branchRoutes)
			}
		})
	}
}

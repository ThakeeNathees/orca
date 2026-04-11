package workflow

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

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
			got := EdgesFromExpr(tt.expr)
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
			BlockBody: ast.BlockBody{Expressions: exprs},
			Name:      name,
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
			rw := Resolve(tt.block, tt.isTrigger, nil)

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
				if e.From == NodeSTART {
					t.Errorf("unexpected START edge: %v", e)
				}
			}
		})
	}
}

func TestExprToNodeName(t *testing.T) {
	tests := []struct {
		name     string
		expr     ast.Expression
		expected string
	}{
		{
			"identifier",
			ident("foo"),
			"foo",
		},
		{
			"member access",
			&ast.MemberAccess{Object: ident("agents"), Member: "researcher"},
			"agents.researcher",
		},
		{
			"subscription",
			&ast.Subscription{Object: ident("agents")},
			"agents[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExprToNodeName(tt.expr)
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
		Name: "pipeline",
		BlockBody: ast.BlockBody{
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
	rw := Resolve(block, isTrigger, nil)

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
		Name: "simple",
		BlockBody: ast.BlockBody{
			Kind: "workflow",
			Expressions: []ast.Expression{
				arrow(ident("A"), ident("B")),
			},
		},
	}

	rw := Resolve(block, nil, nil)

	got := rw.Predecessors("A")
	if len(got) != 0 {
		t.Errorf("Predecessors(A) = %v, want []", got)
	}

	got = rw.Predecessors("B")
	if len(got) != 1 || got[0] != "A" {
		t.Errorf("Predecessors(B) = %v, want [A]", got)
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
		BlockNameAnon: name,
		BlockBody: ast.BlockBody{
			Kind:        "branch",
			Assignments: assignments,
		},
	}
}

// branchLookup builds a getBranchBody function from inline branch BlockExpressions.
func branchLookup(branches ...*ast.BlockExpression) func(string) *ast.BlockBody {
	m := make(map[string]*ast.BlockBody, len(branches))
	for _, b := range branches {
		m[b.BlockNameAnon] = &b.BlockBody
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
			BlockBody: ast.BlockBody{Expressions: exprs},
			Name:      name,
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
			"inline branch with simple routes",
			makeBlock("wf", arrow(ident("A"), simpleBranch)),
			branchLookup(simpleBranch),
			[]string{"A", "B", "C"},
			[]Edge{{From: "B", To: NodeEND}, {From: "C", To: NodeEND}},
			[]string{"A"},
			1,
			2,
		},
		{
			"inline branch with chain routes",
			makeBlock("wf", arrow(ident("A"), chainBranch)),
			branchLookup(chainBranch),
			[]string{"A", "B", "C", "D"},
			[]Edge{{From: "B", To: "C"}, {From: "C", To: NodeEND}, {From: "D", To: NodeEND}},
			[]string{"A"},
			1,
			2,
		},
		{
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
			[]string{"A", "B", "C", "D"},
			[]Edge{{From: "C", To: NodeEND}, {From: "D", To: NodeEND}},
			[]string{"A", "B"},
			1,
			2,
		},
		{
			"branch with complex subgraph in route",
			makeBlock("wf", arrow(ident("A"), complexBranch)),
			branchLookup(complexBranch),
			[]string{"A", "B", "D", "C"},
			[]Edge{{From: "B", To: "D"}, {From: "C", To: "D"}, {From: "D", To: NodeEND}},
			[]string{"A"},
			1,
			2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := Resolve(tt.block, nil, tt.getBranchBody)

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

package graph_test

import (
	"testing"

	"github.com/thakee/orca/compiler/graph"
)

// TestAddNode verifies node insertion, deduplication, and ordering.
func TestAddNode(t *testing.T) {
	tests := []struct {
		name     string
		add      []string
		expected []string
	}{
		{
			name:     "empty graph",
			add:      nil,
			expected: nil,
		},
		{
			name:     "single node",
			add:      []string{"a"},
			expected: []string{"a"},
		},
		{
			name:     "multiple nodes preserve insertion order",
			add:      []string{"c", "a", "b"},
			expected: []string{"c", "a", "b"},
		},
		{
			name:     "duplicate nodes are ignored",
			add:      []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := graph.New[string]()
			for _, n := range tt.add {
				g.AddNode(n)
			}
			got := g.Nodes()
			assertSliceEqual(t, got, tt.expected)
		})
	}
}

// TestHasNode checks node existence queries.
func TestHasNode(t *testing.T) {
	tests := []struct {
		name   string
		add    []string
		query  string
		expect bool
	}{
		{
			name:   "node exists",
			add:    []string{"a", "b"},
			query:  "a",
			expect: true,
		},
		{
			name:   "node does not exist",
			add:    []string{"a", "b"},
			query:  "c",
			expect: false,
		},
		{
			name:   "empty graph",
			add:    nil,
			query:  "a",
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := graph.New[string]()
			for _, n := range tt.add {
				g.AddNode(n)
			}
			if got := g.HasNode(tt.query); got != tt.expect {
				t.Errorf("HasNode(%q) = %v, want %v", tt.query, got, tt.expect)
			}
		})
	}
}

// TestAddEdge verifies edge insertion and implicit node creation.
func TestAddEdge(t *testing.T) {
	tests := []struct {
		name          string
		edges         [][2]string
		expectedNodes []string
		expectedEdges []graph.Edge[string]
	}{
		{
			name:          "single edge creates both nodes",
			edges:         [][2]string{{"a", "b"}},
			expectedNodes: []string{"a", "b"},
			expectedEdges: []graph.Edge[string]{{From: "a", To: "b"}},
		},
		{
			name:          "duplicate edges are ignored",
			edges:         [][2]string{{"a", "b"}, {"a", "b"}},
			expectedNodes: []string{"a", "b"},
			expectedEdges: []graph.Edge[string]{{From: "a", To: "b"}},
		},
		{
			name:          "chain A->B->C",
			edges:         [][2]string{{"a", "b"}, {"b", "c"}},
			expectedNodes: []string{"a", "b", "c"},
			expectedEdges: []graph.Edge[string]{{From: "a", To: "b"}, {From: "b", To: "c"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := graph.New[string]()
			for _, e := range tt.edges {
				g.AddEdge(e[0], e[1])
			}
			assertSliceEqual(t, g.Nodes(), tt.expectedNodes)
			gotEdges := g.Edges()
			if len(gotEdges) != len(tt.expectedEdges) {
				t.Fatalf("Edges() len = %d, want %d", len(gotEdges), len(tt.expectedEdges))
			}
			for i, e := range gotEdges {
				if e != tt.expectedEdges[i] {
					t.Errorf("Edges()[%d] = %v, want %v", i, e, tt.expectedEdges[i])
				}
			}
		})
	}
}

// TestHasEdge checks edge existence queries.
func TestHasEdge(t *testing.T) {
	tests := []struct {
		name   string
		edges  [][2]string
		from   string
		to     string
		expect bool
	}{
		{
			name:   "edge exists",
			edges:  [][2]string{{"a", "b"}},
			from:   "a",
			to:     "b",
			expect: true,
		},
		{
			name:   "reverse edge does not exist",
			edges:  [][2]string{{"a", "b"}},
			from:   "b",
			to:     "a",
			expect: false,
		},
		{
			name:   "no edges",
			edges:  nil,
			from:   "a",
			to:     "b",
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := graph.New[string]()
			for _, e := range tt.edges {
				g.AddEdge(e[0], e[1])
			}
			if got := g.HasEdge(tt.from, tt.to); got != tt.expect {
				t.Errorf("HasEdge(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.expect)
			}
		})
	}
}

// TestSuccessors checks outgoing neighbor queries.
func TestSuccessors(t *testing.T) {
	tests := []struct {
		name     string
		edges    [][2]string
		query    string
		expected []string
	}{
		{
			name:     "node with successors",
			edges:    [][2]string{{"a", "b"}, {"a", "c"}},
			query:    "a",
			expected: []string{"b", "c"},
		},
		{
			name:     "leaf node",
			edges:    [][2]string{{"a", "b"}},
			query:    "b",
			expected: nil,
		},
		{
			name:     "nonexistent node",
			edges:    [][2]string{{"a", "b"}},
			query:    "z",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := graph.New[string]()
			for _, e := range tt.edges {
				g.AddEdge(e[0], e[1])
			}
			assertSliceEqual(t, g.Successors(tt.query), tt.expected)
		})
	}
}

// TestPredecessors checks incoming neighbor queries.
func TestPredecessors(t *testing.T) {
	tests := []struct {
		name     string
		edges    [][2]string
		query    string
		expected []string
	}{
		{
			name:     "node with predecessors",
			edges:    [][2]string{{"a", "c"}, {"b", "c"}},
			query:    "c",
			expected: []string{"a", "b"},
		},
		{
			name:     "entry node",
			edges:    [][2]string{{"a", "b"}},
			query:    "a",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := graph.New[string]()
			for _, e := range tt.edges {
				g.AddEdge(e[0], e[1])
			}
			assertSliceEqual(t, g.Predecessors(tt.query), tt.expected)
		})
	}
}

// TestEntryNodes checks nodes with no incoming edges.
func TestEntryNodes(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []string
		edges    [][2]string
		expected []string
	}{
		{
			name:     "empty graph",
			expected: nil,
		},
		{
			name:     "single node no edges",
			nodes:    []string{"a"},
			expected: []string{"a"},
		},
		{
			name:     "linear chain",
			edges:    [][2]string{{"a", "b"}, {"b", "c"}},
			expected: []string{"a"},
		},
		{
			name:     "diamond",
			edges:    [][2]string{{"a", "b"}, {"a", "c"}, {"b", "d"}, {"c", "d"}},
			expected: []string{"a"},
		},
		{
			name:     "multiple entry nodes",
			edges:    [][2]string{{"a", "c"}, {"b", "c"}},
			expected: []string{"a", "b"},
		},
		{
			name:     "disconnected components",
			edges:    [][2]string{{"a", "b"}, {"c", "d"}},
			expected: []string{"a", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := graph.New[string]()
			for _, n := range tt.nodes {
				g.AddNode(n)
			}
			for _, e := range tt.edges {
				g.AddEdge(e[0], e[1])
			}
			assertSliceEqual(t, g.EntryNodes(), tt.expected)
		})
	}
}

// TestLeafNodes checks nodes with no outgoing edges.
func TestLeafNodes(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []string
		edges    [][2]string
		expected []string
	}{
		{
			name:     "empty graph",
			expected: nil,
		},
		{
			name:     "single node no edges",
			nodes:    []string{"a"},
			expected: []string{"a"},
		},
		{
			name:     "linear chain",
			edges:    [][2]string{{"a", "b"}, {"b", "c"}},
			expected: []string{"c"},
		},
		{
			name:     "multiple leaf nodes",
			edges:    [][2]string{{"a", "b"}, {"a", "c"}},
			expected: []string{"b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := graph.New[string]()
			for _, n := range tt.nodes {
				g.AddNode(n)
			}
			for _, e := range tt.edges {
				g.AddEdge(e[0], e[1])
			}
			assertSliceEqual(t, g.LeafNodes(), tt.expected)
		})
	}
}

// TestTopologicalSort verifies Kahn's algorithm for valid DAGs.
func TestTopologicalSort(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []string
		edges    [][2]string
		expected []string
	}{
		{
			name:     "empty graph",
			expected: nil,
		},
		{
			name:     "single node",
			nodes:    []string{"a"},
			expected: []string{"a"},
		},
		{
			name:     "linear chain",
			edges:    [][2]string{{"a", "b"}, {"b", "c"}},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "diamond",
			edges:    [][2]string{{"a", "b"}, {"a", "c"}, {"b", "d"}, {"c", "d"}},
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name:     "disconnected components preserve insertion order",
			edges:    [][2]string{{"x", "y"}, {"a", "b"}},
			expected: []string{"x", "y", "a", "b"},
		},
		{
			name:     "independent nodes preserve insertion order",
			nodes:    []string{"c", "a", "b"},
			expected: []string{"c", "a", "b"},
		},
		{
			name:     "complex DAG",
			edges:    [][2]string{{"a", "c"}, {"b", "c"}, {"c", "d"}, {"b", "d"}},
			expected: []string{"a", "b", "c", "d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := graph.New[string]()
			for _, n := range tt.nodes {
				g.AddNode(n)
			}
			for _, e := range tt.edges {
				g.AddEdge(e[0], e[1])
			}
			got, err := g.TopologicalSort()
			if err != nil {
				t.Fatalf("TopologicalSort() unexpected error: %v", err)
			}
			assertSliceEqual(t, got, tt.expected)
		})
	}
}

// TestTopologicalSortCycleError verifies that cycles produce errors.
func TestTopologicalSortCycleError(t *testing.T) {
	tests := []struct {
		name  string
		edges [][2]string
	}{
		{
			name:  "simple cycle A->B->A",
			edges: [][2]string{{"a", "b"}, {"b", "a"}},
		},
		{
			name:  "self loop",
			edges: [][2]string{{"a", "a"}},
		},
		{
			name:  "three node cycle",
			edges: [][2]string{{"a", "b"}, {"b", "c"}, {"c", "a"}},
		},
		{
			name:  "cycle with tail",
			edges: [][2]string{{"x", "a"}, {"a", "b"}, {"b", "a"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := graph.New[string]()
			for _, e := range tt.edges {
				g.AddEdge(e[0], e[1])
			}
			_, err := g.TopologicalSort()
			if err == nil {
				t.Fatal("TopologicalSort() expected error for cyclic graph, got nil")
			}
		})
	}
}

// TestHasCycle checks cycle detection.
func TestHasCycle(t *testing.T) {
	tests := []struct {
		name   string
		edges  [][2]string
		expect bool
	}{
		{
			name:   "no cycle",
			edges:  [][2]string{{"a", "b"}, {"b", "c"}},
			expect: false,
		},
		{
			name:   "cycle",
			edges:  [][2]string{{"a", "b"}, {"b", "a"}},
			expect: true,
		},
		{
			name:   "self loop",
			edges:  [][2]string{{"a", "a"}},
			expect: true,
		},
		{
			name:   "empty graph",
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := graph.New[string]()
			for _, e := range tt.edges {
				g.AddEdge(e[0], e[1])
			}
			if got := g.HasCycle(); got != tt.expect {
				t.Errorf("HasCycle() = %v, want %v", got, tt.expect)
			}
		})
	}
}

// TestReachable checks BFS reachability.
func TestReachable(t *testing.T) {
	tests := []struct {
		name     string
		edges    [][2]string
		from     string
		expected []string
	}{
		{
			name:     "linear chain from start",
			edges:    [][2]string{{"a", "b"}, {"b", "c"}},
			from:     "a",
			expected: []string{"b", "c"},
		},
		{
			name:     "linear chain from middle",
			edges:    [][2]string{{"a", "b"}, {"b", "c"}},
			from:     "b",
			expected: []string{"c"},
		},
		{
			name:     "leaf node reaches nothing",
			edges:    [][2]string{{"a", "b"}},
			from:     "b",
			expected: nil,
		},
		{
			name:     "diamond from top",
			edges:    [][2]string{{"a", "b"}, {"a", "c"}, {"b", "d"}, {"c", "d"}},
			from:     "a",
			expected: []string{"b", "c", "d"},
		},
		{
			name:     "nonexistent node",
			edges:    [][2]string{{"a", "b"}},
			from:     "z",
			expected: nil,
		},
		{
			name:     "cycle reachability",
			edges:    [][2]string{{"a", "b"}, {"b", "c"}, {"c", "a"}},
			from:     "a",
			expected: []string{"b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := graph.New[string]()
			for _, e := range tt.edges {
				g.AddEdge(e[0], e[1])
			}
			assertSliceEqual(t, g.Reachable(tt.from), tt.expected)
		})
	}
}

// TestIntegerNodes verifies the generic type parameter works with int keys.
func TestIntegerNodes(t *testing.T) {
	g := graph.New[int]()
	g.AddEdge(1, 2)
	g.AddEdge(2, 3)

	got, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() unexpected error: %v", err)
	}
	expected := []int{1, 2, 3}
	if len(got) != len(expected) {
		t.Fatalf("got %v, want %v", got, expected)
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Errorf("got[%d] = %v, want %v", i, got[i], expected[i])
		}
	}
}

// assertSliceEqual compares two slices for equality.
func assertSliceEqual[T comparable](t *testing.T, got, expected []T) {
	t.Helper()
	if len(got) != len(expected) {
		t.Fatalf("got %v (len %d), want %v (len %d)", got, len(got), expected, len(expected))
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Errorf("index %d: got %v, want %v", i, got[i], expected[i])
		}
	}
}

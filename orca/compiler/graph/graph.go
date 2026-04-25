// Package graph provides a generic directed graph with insertion-ordered nodes.
// It supports topological sorting (Kahn's algorithm), cycle detection, and
// reachability queries. Used by the analyzer for block dependency ordering
// and by the workflow package for control flow graphs.
package graph

import "fmt"

// Edge represents a directed edge from one node to another.
type Edge[K comparable] struct {
	From K
	To   K
}

// Graph is a generic directed graph with insertion-ordered nodes.
// Nodes are identified by keys of type K. Duplicate nodes and edges
// are silently ignored, preserving the first insertion order.
type Graph[K comparable] struct {
	nodes []K            // insertion-ordered node list
	seen  map[K]bool     // dedup set for nodes
	adj   map[K][]K      // outgoing edges (adjacency list)
	inAdj map[K][]K      // incoming edges (reverse adjacency list)
	edges []Edge[K]      // insertion-ordered edge list
	edgeSet map[[2]K]bool // dedup set for edges
}

// New creates an empty directed graph.
func New[K comparable]() *Graph[K] {
	return &Graph[K]{
		seen:    make(map[K]bool),
		adj:     make(map[K][]K),
		inAdj:   make(map[K][]K),
		edgeSet: make(map[[2]K]bool),
	}
}

// AddNode adds a node to the graph. If the node already exists, this is a no-op.
func (g *Graph[K]) AddNode(node K) {
	if g.seen[node] {
		return
	}
	g.seen[node] = true
	g.nodes = append(g.nodes, node)
}

// AddEdge adds a directed edge from -> to. Both nodes are implicitly added
// if they don't already exist. Duplicate edges are ignored.
func (g *Graph[K]) AddEdge(from, to K) {
	g.AddNode(from)
	g.AddNode(to)
	key := [2]K{from, to}
	if g.edgeSet[key] {
		return
	}
	g.edgeSet[key] = true
	g.edges = append(g.edges, Edge[K]{From: from, To: to})
	g.adj[from] = append(g.adj[from], to)
	g.inAdj[to] = append(g.inAdj[to], from)
}

// Nodes returns all nodes in insertion order.
func (g *Graph[K]) Nodes() []K {
	return g.nodes
}

// Edges returns all edges in insertion order.
func (g *Graph[K]) Edges() []Edge[K] {
	return g.edges
}

// HasNode returns true if the node exists in the graph.
func (g *Graph[K]) HasNode(node K) bool {
	return g.seen[node]
}

// HasEdge returns true if the directed edge from -> to exists.
func (g *Graph[K]) HasEdge(from, to K) bool {
	return g.edgeSet[[2]K{from, to}]
}

// Successors returns the outgoing neighbors of a node in insertion order.
// Returns nil if the node has no outgoing edges or doesn't exist.
func (g *Graph[K]) Successors(node K) []K {
	return g.adj[node]
}

// Predecessors returns the incoming neighbors of a node in insertion order.
// Returns nil if the node has no incoming edges or doesn't exist.
func (g *Graph[K]) Predecessors(node K) []K {
	return g.inAdj[node]
}

// EntryNodes returns nodes with no incoming edges, in insertion order.
func (g *Graph[K]) EntryNodes() []K {
	var entries []K
	for _, n := range g.nodes {
		if len(g.inAdj[n]) == 0 {
			entries = append(entries, n)
		}
	}
	return entries
}

// LeafNodes returns nodes with no outgoing edges, in insertion order.
func (g *Graph[K]) LeafNodes() []K {
	var leaves []K
	for _, n := range g.nodes {
		if len(g.adj[n]) == 0 {
			leaves = append(leaves, n)
		}
	}
	return leaves
}

// TopologicalSort returns nodes in topological order using Kahn's algorithm.
// When multiple nodes have in-degree 0, insertion order is used as the
// tie-breaker for deterministic output. Returns an error if the graph
// contains a cycle.
func (g *Graph[K]) TopologicalSort() ([]K, error) {
	if len(g.nodes) == 0 {
		return nil, nil
	}

	// Compute in-degrees.
	inDeg := make(map[K]int, len(g.nodes))
	for _, n := range g.nodes {
		inDeg[n] = len(g.inAdj[n])
	}

	// Seed the queue with all zero-in-degree nodes in insertion order.
	var queue []K
	for _, n := range g.nodes {
		if inDeg[n] == 0 {
			queue = append(queue, n)
		}
	}

	// Index each node by its insertion position so we can sort successors
	// by insertion order when they become ready.
	pos := make(map[K]int, len(g.nodes))
	for i, n := range g.nodes {
		pos[n] = i
	}

	var result []K
	for len(queue) > 0 {
		// Pop front.
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		// For each successor, decrement in-degree. If it reaches 0, insert
		// into queue at the correct position to maintain insertion order.
		for _, succ := range g.adj[node] {
			inDeg[succ]--
			if inDeg[succ] == 0 {
				// Insert into queue maintaining insertion-order sort.
				inserted := false
				for i, q := range queue {
					if pos[succ] < pos[q] {
						queue = append(queue[:i+1], queue[i:]...)
						queue[i] = succ
						inserted = true
						break
					}
				}
				if !inserted {
					queue = append(queue, succ)
				}
			}
		}
	}

	if len(result) != len(g.nodes) {
		return nil, fmt.Errorf("graph contains a cycle (%d nodes in cycle)", len(g.nodes)-len(result))
	}

	return result, nil
}

// HasCycle returns true if the graph contains at least one cycle.
func (g *Graph[K]) HasCycle() bool {
	_, err := g.TopologicalSort()
	return err != nil
}

// Reachable returns all nodes reachable from the given node via BFS,
// excluding the start node itself. Returns nodes in BFS discovery order.
// Returns nil if the node doesn't exist or has no reachable nodes.
func (g *Graph[K]) Reachable(from K) []K {
	if !g.seen[from] {
		return nil
	}

	visited := map[K]bool{from: true}
	queue := []K{from}
	var result []K

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, succ := range g.adj[node] {
			if !visited[succ] {
				visited[succ] = true
				result = append(result, succ)
				queue = append(queue, succ)
			}
		}
	}

	return result
}

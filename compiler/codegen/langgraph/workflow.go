package langgraph

import (
	"fmt"
	"strings"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/codegen/python"
	"github.com/thakee/orca/compiler/token"
)

// LangGraph graph terminal constants used in generated add_edge() calls.
const (
	nodeSTART = "START"
	nodeEND   = "END"
)

// workflowEdge represents a single directed edge between two nodes.
type workflowEdge struct {
	From string // node name, nodeSTART, or nodeEND
	To   string // node name, nodeSTART, or nodeEND
}

// resolvedWorkflow holds the extracted graph structure for a single workflow block.
type resolvedWorkflow struct {
	Name  string
	Nodes []string       // unique node names (excluding START/END), in order of first appearance
	Edges []workflowEdge // all edges in declaration order
}

// resolveWorkflow extracts nodes and edges from a workflow block's arrow expressions.
// After collecting explicit edges, it infers implicit START/END connections:
//   - Any node with no incoming edges gets an implicit START -> node edge
//   - Any node with no outgoing edges gets an implicit node -> END edge
func resolveWorkflow(block *ast.BlockStatement) resolvedWorkflow {
	rw := resolvedWorkflow{Name: block.Name}
	seenNodes := make(map[string]bool)
	seenEdges := make(map[[2]string]bool)

	for _, expr := range block.Expressions {
		edges := flattenEdges(expr)
		for _, e := range edges {
			edgeKey := [2]string{e.From, e.To}
			if seenEdges[edgeKey] {
				continue
			}
			seenEdges[edgeKey] = true
			rw.Edges = append(rw.Edges, e)

			for _, name := range []string{e.From, e.To} {
				if name != nodeSTART && name != nodeEND && !seenNodes[name] {
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
func inferTerminals(nodes []string, edges []workflowEdge) []workflowEdge {
	hasIncoming := make(map[string]bool)
	hasOutgoing := make(map[string]bool)

	for _, e := range edges {
		if e.To != nodeEND {
			hasIncoming[e.To] = true
		}
		if e.From != nodeSTART {
			hasOutgoing[e.From] = true
		}
	}

	// Prepend implicit START edges (before existing edges for natural ordering).
	var startEdges []workflowEdge
	for _, node := range nodes {
		if !hasIncoming[node] {
			startEdges = append(startEdges, workflowEdge{From: nodeSTART, To: node})
		}
	}

	// Append implicit END edges.
	var endEdges []workflowEdge
	for _, node := range nodes {
		if !hasOutgoing[node] {
			endEdges = append(endEdges, workflowEdge{From: node, To: nodeEND})
		}
	}

	result := make([]workflowEdge, 0, len(startEdges)+len(edges)+len(endEdges))
	result = append(result, startEdges...)
	result = append(result, edges...)
	result = append(result, endEdges...)
	return result
}

// flattenEdges walks a (possibly chained) arrow expression and returns
// the list of individual edges. For example, A -> B -> C yields:
// [{A, B}, {B, C}].
func flattenEdges(expr ast.Expression) []workflowEdge {
	bin, ok := expr.(*ast.BinaryExpression)
	if !ok || bin.Operator.Type != token.ARROW {
		return nil
	}

	// The parser builds left-associative trees: ((A -> B) -> C).
	// Recursively flatten the left side, then connect its last node to the right.
	leftEdges := flattenEdges(bin.Left)

	rightName := nodeName(bin.Right)

	if len(leftEdges) > 0 {
		// Connect the last node from the left chain to the right node.
		lastTo := leftEdges[len(leftEdges)-1].To
		return append(leftEdges, workflowEdge{From: lastTo, To: rightName})
	}

	// Base case: simple A -> B.
	leftName := nodeName(bin.Left)
	return []workflowEdge{{From: leftName, To: rightName}}
}

// nodeName extracts the node name string from an expression.
func nodeName(expr ast.Expression) string {
	if ident, ok := expr.(*ast.Identifier); ok {
		return ident.Value
	}
	// The analyzer guarantees only identifiers reach here.
	return ""
}

// collectWorkflows returns all workflow blocks. Cached so that both
// writeWorkflowSection and workflowImports share the same traversal.
func (b *LangGraphBackend) collectWorkflows() []*ast.BlockStatement {
	return b.CollectBlocksByKind(token.BlockWorkflow)
}

// writeWorkflowSection emits all workflow blocks as LangGraph StateGraph
// construction code. Each workflow generates:
//   - Node wrapper functions for each referenced agent/tool
//   - StateGraph instantiation with add_node/add_edge calls
//   - Compilation to a runnable graph
func (b *LangGraphBackend) writeWorkflowSection(s *strings.Builder, blocks []*ast.BlockStatement) {
	var resolved []resolvedWorkflow
	for _, block := range blocks {
		rw := resolveWorkflow(block)
		if len(rw.Nodes) > 0 {
			resolved = append(resolved, rw)
		}
	}
	if len(resolved) == 0 {
		return
	}

	s.WriteString("\n# --- Workflows ---\n")

	for _, rw := range resolved {
		b.writeWorkflow(s, rw)
	}
}

// writeWorkflow emits the LangGraph code for a single workflow.
func (b *LangGraphBackend) writeWorkflow(s *strings.Builder, rw resolvedWorkflow) {
	for _, node := range rw.Nodes {
		s.WriteString("\n")
		fmt.Fprintf(s, "def _node_%s(state: GraphState) -> dict:\n", node)
		fmt.Fprintf(s, "    \"\"\"Workflow node wrapping '%s'.\"\"\"\n", node)
		fmt.Fprintf(s, "    pass  # TODO: implement node invocation for '%s'\n", node)
	}

	s.WriteString("\n")
	fmt.Fprintf(s, "%s = StateGraph(GraphState)\n", rw.Name)

	for _, node := range rw.Nodes {
		fmt.Fprintf(s, "%s.add_node(\"%s\", _node_%s)\n", rw.Name, node, node)
	}

	for _, edge := range rw.Edges {
		fmt.Fprintf(s, "%s.add_edge(%s, %s)\n", rw.Name, edgeEndpoint(edge.From), edgeEndpoint(edge.To))
	}

	fmt.Fprintf(s, "%s = %s.compile()\n", rw.Name, rw.Name)
}

// edgeEndpoint returns the Python expression for a node reference in an
// add_edge call. START and END map to the LangGraph constants; regular
// nodes are quoted strings.
func edgeEndpoint(name string) string {
	if name == nodeSTART || name == nodeEND {
		return name
	}
	return fmt.Sprintf("%q", name)
}

// workflowImports returns the Python imports required by the given workflow blocks.
func workflowImports(blocks []*ast.BlockStatement) []python.PythonImport {
	hasEdges := false
	for _, block := range blocks {
		if len(block.Expressions) > 0 {
			hasEdges = true
			break
		}
	}
	if !hasEdges {
		return nil
	}

	return []python.PythonImport{{
		Module:     "langgraph.graph",
		FromImport: true,
		Symbols: []python.ImportSymbol{
			{Name: "StateGraph"},
			{Name: "START"},
			{Name: "END"},
		},
	}}
}

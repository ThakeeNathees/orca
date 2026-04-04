package langgraph

import (
	"fmt"
	"strings"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/codegen/python"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/workflow"
)

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
	var resolved []workflow.ResolvedWorkflow
	for _, block := range blocks {
		rw := workflow.Resolve(block)
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
func (b *LangGraphBackend) writeWorkflow(s *strings.Builder, rw workflow.ResolvedWorkflow) {
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
	if name == workflow.NodeSTART || name == workflow.NodeEND {
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

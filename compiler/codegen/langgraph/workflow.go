package langgraph

import (
	"fmt"
	"strings"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/codegen/python"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
	"github.com/thakee/orca/compiler/workflow"
)

// collectWorkflows returns all workflow blocks. Cached so that both
// writeWorkflowSection and workflowImports share the same traversal.
func (b *LangGraphBackend) collectWorkflows() []*ast.BlockStatement {
	return b.CollectBlocksByKind(token.BlockWorkflow)
}

// triggerPredicate returns a function that checks if a node name is a trigger
// block kind, using the program's symbol table.
func (b *LangGraphBackend) triggerPredicate() func(string) bool {
	return func(name string) bool {
		sym, ok := b.Program.SymbolTable.LookupSymbol(name)
		if !ok {
			return false
		}
		return sym.Type.Kind == types.BlockRef && sym.Type.BlockKind.IsTrigger()
	}
}

// writeWorkflowSection emits all workflow blocks as LangGraph StateGraph
// construction code. Each workflow generates:
//   - Node wrapper functions for each referenced agent/tool
//   - A router function that dispatches to entry nodes based on trigger source
//   - StateGraph instantiation with add_node/add_conditional_edges/add_edge calls
//   - Compilation to a runnable graph
func (b *LangGraphBackend) writeWorkflowSection(s *strings.Builder, blocks []*ast.BlockStatement) {
	isTrigger := b.triggerPredicate()
	var resolved []workflow.ResolvedWorkflow
	for _, block := range blocks {
		rw := workflow.Resolve(block, isTrigger)
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
	// Node wrapper functions (processing nodes only, not triggers).
	for _, node := range rw.Nodes {
		s.WriteString("\n")
		fmt.Fprintf(s, "def _node_%s(state: GraphState) -> dict:\n", node)
		fmt.Fprintf(s, "    \"\"\"Workflow node wrapping '%s'.\"\"\"\n", node)
		fmt.Fprintf(s, "    pass  # TODO: implement node invocation for '%s'\n", node)
	}

	// Router function.
	s.WriteString("\n")
	writeRouter(s, rw)

	// StateGraph construction.
	s.WriteString("\n")
	fmt.Fprintf(s, "%s = StateGraph(GraphState)\n", rw.Name)

	for _, node := range rw.Nodes {
		fmt.Fprintf(s, "%s.add_node(\"%s\", _node_%s)\n", rw.Name, node, node)
	}

	// START → router (always conditional edges).
	fmt.Fprintf(s, "%s.add_conditional_edges(START, _route_%s)\n", rw.Name, rw.Name)

	// Processing edges + END edges.
	for _, edge := range rw.Edges {
		fmt.Fprintf(s, "%s.add_edge(%s, %s)\n", rw.Name, edgeEndpoint(edge.From), edgeEndpoint(edge.To))
	}

	fmt.Fprintf(s, "%s = %s.compile()\n", rw.Name, rw.Name)
}

// writeRouter emits the _route_<workflow> function that dispatches to entry
// nodes based on the trigger source in state["__orca_trigger__"].
func writeRouter(s *strings.Builder, rw workflow.ResolvedWorkflow) {
	fanOut := rw.IsFanOut()
	returnType := "str"
	if fanOut {
		returnType = "list[str]"
	}

	fmt.Fprintf(s, "def _route_%s(state: GraphState) -> %s:\n", rw.Name, returnType)
	fmt.Fprintf(s, "    \"\"\"Route to entry node based on trigger source.\"\"\"\n")

	if !rw.HasTriggers() {
		// No triggers: return the entry node(s) unconditionally.
		fmt.Fprintf(s, "    return %s\n", routeReturn(rw.EntryNodes, fanOut))
		return
	}

	// Trigger-based routing.
	fmt.Fprintf(s, "    trigger = state.get(\"__orca_trigger__\")\n")
	for _, trig := range rw.Triggers {
		entries := rw.TriggerMap[trig]
		fmt.Fprintf(s, "    if trigger == %q:\n", trig)
		fmt.Fprintf(s, "        return %s\n", routeReturn(entries, fanOut))
	}

	// Unknown trigger: fail loudly at runtime.
	fmt.Fprintf(s, "    raise ValueError(f\"unknown trigger: {trigger!r}\")\n")
}

// routeReturn formats the return value for a router function.
// For a single node without fan-out returns a quoted string, otherwise a list.
func routeReturn(nodes []string, fanOut bool) string {
	if len(nodes) == 1 && !fanOut {
		return fmt.Sprintf("%q", nodes[0])
	}
	quoted := make([]string, len(nodes))
	for i, n := range nodes {
		quoted[i] = fmt.Sprintf("%q", n)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
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

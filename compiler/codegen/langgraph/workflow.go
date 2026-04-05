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
//   - A per-workflow TypedDict state class with typed node fields
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

// nodeFuncName returns the Python function name for a workflow node wrapper.
func nodeFuncName(nodeName string) string {
	return orcaPrefix + "node_" + nodeName
}

// routeFuncName returns the Python function name for a workflow router.
func routeFuncName(workflowName string) string {
	return orcaPrefix + "route_" + workflowName
}

// stateClassName returns the TypedDict class name for a workflow's state.
func stateClassName(workflowName string) string {
	return orcaPrefix + "state_" + workflowName
}

// writeWorkflow emits the LangGraph code for a single workflow.
func (b *LangGraphBackend) writeWorkflow(s *strings.Builder, rw workflow.ResolvedWorkflow) {
	stateName := stateClassName(rw.Name)

	// Look up each node's block once for both state type and node function generation.
	nodeBlocks := make(map[string]*ast.BlockStatement, len(rw.Nodes))
	for _, node := range rw.Nodes {
		nodeBlocks[node] = b.Program.Ast.FindBlockWithName(node)
	}

	writeWorkflowState(s, rw, stateName, nodeBlocks)

	for _, node := range rw.Nodes {
		s.WriteString("\n")
		b.writeNodeFunc(s, rw, node, stateName, nodeBlocks[node])
	}

	s.WriteString("\n")
	writeRouter(s, rw, stateName)

	s.WriteString("\n")
	fmt.Fprintf(s, "%s = StateGraph(%s)\n", rw.Name, stateName)

	for _, node := range rw.Nodes {
		fmt.Fprintf(s, "%s.add_node(\"%s\", %s)\n", rw.Name, node, nodeFuncName(node))
	}

	fmt.Fprintf(s, "%s.add_conditional_edges(START, %s)\n", rw.Name, routeFuncName(rw.Name))

	for _, edge := range rw.Edges {
		fmt.Fprintf(s, "%s.add_edge(%s, %s)\n", rw.Name, edgeEndpoint(edge.From), edgeEndpoint(edge.To))
	}

	fmt.Fprintf(s, "%s = %s.compile()\n", rw.Name, rw.Name)
}

// writeWorkflowState emits a per-workflow TypedDict class with typed fields
// for trigger metadata and each processing node's output.
func writeWorkflowState(s *strings.Builder, rw workflow.ResolvedWorkflow, stateName string, nodeBlocks map[string]*ast.BlockStatement) {
	s.WriteString("\n")
	fmt.Fprintf(s, "class %s(TypedDict):\n", stateName)
	fmt.Fprintf(s, "    %s: str | None\n", orcaTriggerField)
	fmt.Fprintf(s, "    %s: dict | None\n", orcaPayloadField)

	for _, node := range rw.Nodes {
		fmt.Fprintf(s, "    %s: %s\n", node, nodeFieldType(nodeBlocks[node]))
	}
}

// writeNodeFunc emits a single workflow node wrapper function with gather + invoke + return.
func (b *LangGraphBackend) writeNodeFunc(s *strings.Builder, rw workflow.ResolvedWorkflow, node, stateName string, block *ast.BlockStatement) {
	fmt.Fprintf(s, "def %s(state: %s) -> dict:\n", nodeFuncName(node), stateName)
	fmt.Fprintf(s, "    \"\"\"Workflow node wrapping '%s'.\"\"\"\n", node)

	preds := rw.Predecessors(node)
	if len(preds) == 0 {
		fmt.Fprintf(s, "    input_data = state[\"%s\"]\n", orcaPayloadField)
	} else {
		fmt.Fprintf(s, "    input_data = %s(state, [", orcaGatherFunc)
		for i, p := range preds {
			if i > 0 {
				s.WriteString(", ")
			}
			fmt.Fprintf(s, "%q", p)
		}
		s.WriteString("])\n")
	}

	invokeFunc := nodeInvokeFunc(block)
	fmt.Fprintf(s, "    result = %s(%s, input_data)\n", invokeFunc, node)
	fmt.Fprintf(s, "    return {%q: result}\n", node)
}

// nodeInvokeFunc returns the runtime invoke function name for a node.
func nodeInvokeFunc(block *ast.BlockStatement) string {
	if block != nil && block.Kind == token.BlockTool {
		return orcaInvokeToolFunc
	}
	return orcaInvokeAgentFunc
}

// nodeFieldType determines the Python type annotation for a node's output
// field in the workflow state based on its output_schema.
func nodeFieldType(block *ast.BlockStatement) string {
	if block == nil {
		return "Any"
	}
	if schemaExpr, ok := block.GetFieldExpression("output_schema"); ok {
		// Bootstrap mode (nil symbol table) resolves the schema name as a type.
		schemaType := types.ExprType(schemaExpr, nil)
		typeName := python.OrcaTypeToPythonTypeName(schemaType)
		return typeName + " | None"
	}
	return "Any"
}

// writeRouter emits the __orca_route_<workflow> function that dispatches to entry
// nodes based on the trigger source in state["__orca_trigger"].
func writeRouter(s *strings.Builder, rw workflow.ResolvedWorkflow, stateName string) {
	fanOut := rw.IsFanOut()
	returnType := "str"
	if fanOut {
		returnType = "list[str]"
	}

	fmt.Fprintf(s, "def %s(state: %s) -> %s:\n", routeFuncName(rw.Name), stateName, returnType)
	fmt.Fprintf(s, "    \"\"\"Route to entry node based on trigger source.\"\"\"\n")

	if !rw.HasTriggers() {
		// No triggers: return the entry node(s) unconditionally.
		fmt.Fprintf(s, "    return %s\n", routeReturn(rw.EntryNodes, fanOut))
		return
	}

	// Trigger-based routing.
	fmt.Fprintf(s, "    trigger = state.get(\"%s\")\n", orcaTriggerField)
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

	return []python.PythonImport{
		{
			Module:     "langgraph.graph",
			FromImport: true,
			Symbols: []python.ImportSymbol{
				{Name: "StateGraph"},
				{Name: "START"},
				{Name: "END"},
			},
		},
		{
			Module:     "langchain.agents",
			Package:    "langchain",
			FromImport: true,
			Symbols: []python.ImportSymbol{
				{Name: "create_agent"},
			},
		},
	}
}

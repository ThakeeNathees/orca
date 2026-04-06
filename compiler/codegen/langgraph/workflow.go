package langgraph

import (
	"fmt"
	"strings"

	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/codegen/python"
	"github.com/thakee/orca/compiler/helper"
	"github.com/thakee/orca/compiler/types"
	"github.com/thakee/orca/compiler/workflow"
)

// collectWorkflows returns all workflow blocks. Cached so that both
// writeWorkflowSection and workflowImports share the same traversal.
func (b *LangGraphBackend) collectWorkflows() []*ast.BlockStatement {
	return b.CollectBlocksByKind(analyzer.BlockKindWorkflow)
}

// triggerPredicate returns a function that checks if a node name is a trigger
// block kind, using the program's symbol table.
func (b *LangGraphBackend) triggerPredicate() func(string) bool {
	return func(kind string) bool {
		sym, ok := b.Program.SymbolTable.LookupSymbol(kind)
		if !ok {
			return false
		}
		schema, ok := types.LookupBlockSchema(sym.Type)
		if !ok {
			return false
		}
		return helper.HasAnnotation(schema.Annotations, analyzer.AnnotationTriggerNode)
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

// stateClassName returns the TypedDict class name for a workflow's state.
func stateClassName(workflowName string) string {
	return orcaPrefix + "state_" + workflowName
}

// writeWorkflow emits the LangGraph code for a single workflow.
func (b *LangGraphBackend) writeWorkflow(s *strings.Builder, rw workflow.ResolvedWorkflow) {
	stateName := stateClassName(rw.Name)

	// Per-workflow typed state class.
	b.writeWorkflowState(s, rw, stateName)

	// Node wrapper functions (processing nodes only, not triggers).
	for _, node := range rw.Nodes {
		s.WriteString("\n")
		fmt.Fprintf(s, "def %s(state: %s) -> dict:\n", nodeFuncName(node), stateName)
		fmt.Fprintf(s, "    \"\"\"Workflow node wrapping '%s'.\"\"\"\n", node)
		fmt.Fprintf(s, "    pass  # TODO: implement node invocation for '%s'\n", node)
	}

	// Router function.
	s.WriteString("\n")
	writeRouter(s, rw, stateName)

	// StateGraph construction.
	s.WriteString("\n")
	fmt.Fprintf(s, "%s = StateGraph(%s)\n", rw.Name, stateName)

	for _, node := range rw.Nodes {
		fmt.Fprintf(s, "%s.add_node(\"%s\", %s)\n", rw.Name, node, nodeFuncName(node))
	}

	// START → router (always conditional edges).
	fmt.Fprintf(s, "%s.add_conditional_edges(START, %sroute_%s)\n", rw.Name, orcaPrefix, rw.Name)

	// Processing edges + END edges.
	for _, edge := range rw.Edges {
		fmt.Fprintf(s, "%s.add_edge(%s, %s)\n", rw.Name, edgeEndpoint(edge.From), edgeEndpoint(edge.To))
	}

	fmt.Fprintf(s, "%s = %s.compile()\n", rw.Name, rw.Name)
}

// writeWorkflowState emits a per-workflow TypedDict class with typed fields
// for trigger metadata and each processing node's output.
func (b *LangGraphBackend) writeWorkflowState(s *strings.Builder, rw workflow.ResolvedWorkflow, stateName string) {
	s.WriteString("\n")
	fmt.Fprintf(s, "class %s(TypedDict):\n", stateName)
	fmt.Fprintf(s, "    %s: str | None\n", orcaTriggerField)
	fmt.Fprintf(s, "    %s: dict | None\n", orcaPayloadField)

	for _, node := range rw.Nodes {
		typeAnnotation := b.nodeFieldType(node)
		fmt.Fprintf(s, "    %s: %s\n", node, typeAnnotation)
	}
}

// nodeFieldType determines the Python type annotation for a node's output
// field in the workflow state. Rules:
//   - Agent without output_schema → "str | None"
//   - Agent with output_schema → "<schema_name> | None"
//   - Tool without output_schema → "dict | None"
//   - Tool with output_schema → "<schema_name> | None"
//   - Unknown → "str | None"
func (b *LangGraphBackend) nodeFieldType(nodeName string) string {
	block := b.Program.Ast.FindBlockWithName(nodeName)
	if block == nil {
		return "Any"
	}

	// Check for output_schema field.
	if schemaExpr, ok := block.GetFieldExpression("output_schema"); ok {
		// Use bootstrap mode (nil symbol table) to resolve the schema name
		// as a type, since the symbol table stores schemas without names.
		schemaType := types.ExprType(schemaExpr, nil)
		typeName := python.OrcaTypeToPythonTypeName(schemaType)
		return typeName + " | None"
	}

	// No output_schema — output type is unknown.
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

	fmt.Fprintf(s, "def %sroute_%s(state: %s) -> %s:\n", orcaPrefix, rw.Name, stateName, returnType)
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

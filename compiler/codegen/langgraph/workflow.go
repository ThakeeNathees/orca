package langgraph

import (
	"fmt"
	"strings"

	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/codegen/python"
	"github.com/thakee/orca/compiler/types"
	"github.com/thakee/orca/compiler/workflow"
)

// resolveWorkflows pre-processes all workflow blocks, resolving their node
// graphs and trigger predicates.
func (b *LangGraphBackend) resolveWorkflows() {
	wfs := b.collectWorkflows()
	if len(wfs) == 0 {
		return
	}
	b.resolvedWorkflows = make(map[string]workflow.ResolvedWorkflow, len(wfs))
	for _, wf := range wfs {
		rw := workflow.Resolve(wf, b.triggerPredicate(), b.branchBodyLookup())
		b.resolvedWorkflows[wf.Name] = rw
	}
}

// collectWorkflows returns all workflow blocks. Cached so that both
// writeWorkflowSection and workflowImports share the same traversal.
func (b *LangGraphBackend) collectWorkflows() []*ast.BlockStatement {
	return b.CollectBlocksByKind(analyzer.BlockKindWorkflow)
}

// triggerPredicate returns a function that checks if a node name is a trigger
// block kind, using the program's symbol table.
func (b *LangGraphBackend) triggerPredicate() func(string) bool {
	return func(blockName string) bool {
		typ, ok := b.Program.SymbolTable.Lookup(blockName)
		if !ok {
			return false
		}
		return types.IsAnnotated(typ, analyzer.AnnotationTriggerNode)
	}
}

// branchBodyLookup returns a function that looks up a block name in the symbol
// table and returns its BlockBody if it's a branch block, or nil otherwise.
// Works for both named branch blocks and inline branch expressions (registered
// by the type system as __anon_N).
func (b *LangGraphBackend) branchBodyLookup() func(string) *ast.BlockBody {
	return func(name string) *ast.BlockBody {
		typ, ok := b.Program.SymbolTable.Lookup(name)
		if !ok {
			return nil
		}
		if typ.Block == nil || typ.Block.Ast == nil {
			return nil
		}
		if typ.Block.Ast.Kind != workflow.BlockKindBranch {
			return nil
		}
		return typ.Block.Ast
	}
}

// writeWorkflowEntrypoint emits a default Python entrypoint for a single workflow.
// This allows running the generated `main.py` directly (e.g. `python main.py "payload"`).
// Skips when there is no workflow, multiple workflows, or the sole workflow has no
// runnable nodes (same criterion as writeWorkflowSection).
func (b *LangGraphBackend) writeWorkflowEntrypoint(s *strings.Builder) {
	wfs := b.resolvedWorkflows

	if len(wfs) == 0 {
		return
	}
	if len(wfs) >= 2 {
		return
	}

	var rw workflow.ResolvedWorkflow
	for _, r := range wfs {
		rw = r
		break
	}
	if len(rw.Nodes) == 0 {
		return
	}

	s.WriteString("\n")
	s.WriteString("if __name__ == \"__main__\":\n")
	s.WriteString("    payload = sys.argv[1] if len(sys.argv) >= 2 else \"\"\n")
	fmt.Fprintf(s, "    initial_state: %s = {\n", stateClassName(rw.Name))
	fmt.Fprintf(s, "        %q: %q,\n", orcaTriggerField, "")
	fmt.Fprintf(s, "        %q: payload,\n", orcaPayloadField)
	for _, node := range rw.Nodes {
		fmt.Fprintf(s, "        %q: %q,\n", node, "")
	}
	for _, branch := range rw.Branches {
		fmt.Fprintf(s, "        %q: %q,\n", orcaBranchRouteField(branch.Name), "")
	}
	s.WriteString("    }\n")
	fmt.Fprintf(s, "    final_state = %s.invoke(initial_state)\n", rw.Name)
	s.WriteString("    print(final_state)\n")
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
		b.writeWorkflowNode(s, rw, stateName, node)
	}

	// Router function.
	s.WriteString("\n")
	writeRouter(s, rw, stateName)

	// Branch router functions.
	for i, branch := range rw.Branches {
		s.WriteString("\n")
		writeBranchRouter(s, rw, stateName, branch, i)
	}

	// StateGraph construction.
	s.WriteString("\n")
	fmt.Fprintf(s, "%s = StateGraph(%s)\n", rw.Name, stateName)

	for _, node := range rw.Nodes {
		fmt.Fprintf(s, "%s.add_node(\"%s\", %s)\n", rw.Name, node, nodeFuncName(node))
	}

	// START → router (always conditional edges).
	fmt.Fprintf(s, "%s.add_conditional_edges(START, %sroute_%s)\n", rw.Name, orcaPrefix, rw.Name)

	// Branch conditional edges. The source is the branch node itself — its
	// node function stores the computed route key in state, and its router
	// (writeBranchRouter) reads that key to dispatch to a target. See
	// writeBranchRouteMap for the route map shape.
	for i, branch := range rw.Branches {
		routerName := branchRouterFuncName(rw.Name, i)
		fmt.Fprintf(s, "%s.add_conditional_edges(%q, %s", rw.Name, branch.Name, routerName)
		writeBranchRouteMap(s, branch)
		s.WriteString(")\n")
	}

	// Processing edges + END edges.
	for _, edge := range rw.Edges {
		fmt.Fprintf(s, "%s.add_edge(%s, %s)\n", rw.Name, edgeEndpoint(edge.From), edgeEndpoint(edge.To))
	}

	fmt.Fprintf(s, "%s = %s.compile()\n", rw.Name, rw.Name)
}

// writeWorkflowState emits a per-workflow TypedDict class with typed fields
// for trigger metadata, each processing node's output, and each branch's
// route key (stored under orcaBranchRouteField(branch.Name) by the branch's
// node function and read by its router function).
func (b *LangGraphBackend) writeWorkflowState(s *strings.Builder, rw workflow.ResolvedWorkflow, stateName string) {
	s.WriteString("\n")
	fmt.Fprintf(s, "class %s(TypedDict):\n", stateName)
	fmt.Fprintf(s, "    %s: str | None\n", orcaTriggerField)
	fmt.Fprintf(s, "    %s: dict | None\n", orcaPayloadField)

	for _, node := range rw.Nodes {
		typeAnnotation := b.nodeFieldType(node)
		fmt.Fprintf(s, "    %s: %s\n", node, typeAnnotation)
	}

	// Branch route-key fields. Stores the key returned by the branch's
	// transform so the branch router can dispatch conditional edges.
	// TODO: strict typing based on the user-declared route key types
	// (string | number | bool per the branch schema). For now, Any.
	for _, branch := range rw.Branches {
		fmt.Fprintf(s, "    %s: Any\n", orcaBranchRouteField(branch.Name))
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
		schemaType := types.EvalType(schemaExpr, b.Program.SymbolTable)
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

// pythonStringListLiteral formats ss as a Python list of string literals, e.g. ["a", "b"].
func pythonStringListLiteral(ss []string) string {
	if len(ss) == 0 {
		return "[]"
	}
	quoted := make([]string, len(ss))
	for i, s := range ss {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// writeWorkflowNode emits a full Python workflow node function: signature,
// docstring, and body that gathers upstream inputs, dispatches on the block
// kind, and returns the partial state update for this node's output field.
//
// Supported block kinds:
//
//   - agent / tool: invoked via the runtime helpers; output stored under the
//     node's name.
//   - branch: computes a route key from the input (via the branch's transform
//     or pass-through) and returns both the passthrough input under the
//     branch's name AND the route key under orcaBranchRouteField(name). The
//     branch's own router function (see writeBranchRouter) reads the route
//     key to dispatch conditional edges.
func (b *LangGraphBackend) writeWorkflowNode(s *strings.Builder, rw workflow.ResolvedWorkflow, stateName, node string) {
	s.WriteString("\n")
	fmt.Fprintf(s, "def %s(state: %s) -> dict:\n", nodeFuncName(node), stateName)
	fmt.Fprintf(s, "    \"\"\"Workflow node wrapping '%s'.\"\"\"\n", node)

	preds := rw.Predecessors(node)
	fmt.Fprintf(s, "    _predecessors = %s\n", pythonStringListLiteral(preds))
	fmt.Fprintf(s, "    _input = %s(state, _predecessors)\n", orcaGatherFunc)

	// Branches don't have a top-level block statement (they can be inline),
	// so resolve them via rw.FindBranch before falling back to the symbol
	// table / AST lookup used for named blocks like agent/tool.
	if branch := rw.FindBranch(node); branch != nil {
		writeBranchNodeBody(s, branch)
		return
	}

	block := b.Program.Ast.FindBlockWithName(node)
	if block == nil {
		fmt.Fprintf(s, "    raise RuntimeError(\"workflow node '%s': no block definition found\")\n", node)
		return
	}

	switch block.Kind {
	case analyzer.BlockKindAgent:
		fmt.Fprintf(s, "    _out = %s(%s, _input)\n", orcaInvokeAgentFunc, node)
	case analyzer.BlockKindTool:
		fmt.Fprintf(s, "    _out = %s(%s, _input)\n", orcaInvokeToolFunc, node)
	default:
		fmt.Fprintf(s, "    raise NotImplementedError(\"workflow node '%s': block kind '%s' is not supported in workflows yet\")\n", node, block.Kind)
	}
	fmt.Fprintf(s, "    return {%q: _out}\n", node)
}

// branchRouterFuncName returns the Python function name for a branch router.
func branchRouterFuncName(workflowName string, branchIndex int) string {
	return fmt.Sprintf("%sroute_%s_branch_%d", orcaPrefix, workflowName, branchIndex)
}

// writeBranchRouter emits a Python function that reads a branch's route key
// from workflow state and returns it to LangGraph for conditional edge
// dispatch. The key is filtered against the branch's known route keys; any
// unknown key falls back to workflow.BranchRouteKeyDefault. The route map
// always contains the default key (auto-wired to END if the user didn't
// provide one — see writeBranchRouteMap), so LangGraph never receives an
// unknown key at runtime.
func writeBranchRouter(s *strings.Builder, rw workflow.ResolvedWorkflow, stateName string, branch workflow.Branch, branchIndex int) {
	funcName := branchRouterFuncName(rw.Name, branchIndex)
	fmt.Fprintf(s, "def %s(state: %s) -> Any:\n", funcName, stateName)
	fmt.Fprintf(s, "    \"\"\"Branch router for %q.\"\"\"\n", branch.Name)
	fmt.Fprintf(s, "    _key = state.get(%q, %q)\n", orcaBranchRouteField(branch.Name), workflow.BranchRouteKeyDefault)

	knownKeys := make([]string, 0, len(branch.Routes))
	for _, route := range branch.Routes {
		if len(route.EntryNodes) == 0 {
			continue
		}
		knownKeys = append(knownKeys, exprToSource(route.Key))
	}
	if len(knownKeys) > 0 {
		fmt.Fprintf(s, "    if _key in {%s}:\n", strings.Join(knownKeys, ", "))
		s.WriteString("        return _key\n")
	}
	fmt.Fprintf(s, "    return %q\n", workflow.BranchRouteKeyDefault)
}

// writeBranchNodeBody emits the body of a branch's workflow node function
// (called from writeWorkflowNode's branch case). The body:
//
//  1. Uses the already-emitted `_input` (gathered from predecessors).
//  2. Applies the branch's transform to produce a route key — or passes
//     _input through unchanged if there is no transform.
//  3. Returns a state update with the passthrough input under the branch's
//     own name AND the route key under orcaBranchRouteField(branch.Name).
//
// The caller is responsible for writing the function signature, docstring,
// and the _predecessors / _input preamble.
func writeBranchNodeBody(s *strings.Builder, branch *workflow.Branch) {
	if branch.Transform != nil {
		fmt.Fprintf(s, "    _route_key = (%s)(_input)\n", exprToSource(branch.Transform))
	} else {
		s.WriteString("    _route_key = _input\n")
	}
	fmt.Fprintf(s, "    return {%q: _input, %q: _route_key}\n", branch.Name, orcaBranchRouteField(branch.Name))
}

// writeBranchRouteMap emits the route map argument for an add_conditional_edges
// call: `, {key1: target1, ..., "default": <fallback>}`. Routes with empty
// EntryNodes are skipped (unsupported route value shapes). If the user did
// not provide a workflow.BranchRouteKeyDefault entry, one is auto-wired to
// END so LangGraph always has a fallback.
func writeBranchRouteMap(s *strings.Builder, branch workflow.Branch) {
	s.WriteString(", {")
	first := true
	hasDefault := false
	for _, route := range branch.Routes {
		if len(route.EntryNodes) == 0 {
			continue
		}
		if !first {
			s.WriteString(", ")
		}
		first = false
		fmt.Fprintf(s, "%s: %q", exprToSource(route.Key), route.EntryNodes[0])
		if strLit, ok := route.Key.(*ast.StringLiteral); ok && strLit.Value == workflow.BranchRouteKeyDefault {
			hasDefault = true
		}
	}
	if !hasDefault {
		if !first {
			s.WriteString(", ")
		}
		fmt.Fprintf(s, "%q: %s", workflow.BranchRouteKeyDefault, workflow.NodeEND)
	}
	s.WriteString("}")
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

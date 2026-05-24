package agentgraph

import (
	"github.com/thakee/orca/orca/compiler/analyzer"
	"github.com/thakee/orca/orca/compiler/workflow"
	"github.com/thakee/orca/orca/runtime/value"
)

// resolveBlockName maps a workflow node name to the defining block symbol name.
func resolveBlockName(rw *workflow.ResolvedWorkflow, nodeName string) string {
	if rw == nil {
		return nodeName
	}
	if b, ok := rw.NodeToBlockName[nodeName]; ok && b != "" {
		return b
	}
	return nodeName
}

func collectWorkflowNames(rw *workflow.ResolvedWorkflow) []string {
	if rw == nil {
		return nil
	}
	seen := make(map[string]bool)
	var names []string
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		names = append(names, s)
	}
	for _, n := range rw.Nodes {
		add(n)
	}
	for _, t := range rw.Triggers {
		add(t)
	}
	return names
}

// materializeBlock folds a named block's assignments into an immutable runtime Value.
func materializeBlock(ap *analyzer.AnalyzedProgram, blockName string) (value.Value, bool) {
	if ap == nil {
		return value.Value{}, false
	}
	typ, ok := ap.SymbolTable.Lookup(blockName)
	if !ok || typ.Block == nil || typ.Block.Ast == nil {
		return value.Value{}, false
	}
	kind := ""
	if typ.Block.Schema != nil {
		kind = typ.Block.Schema.BlockName
	}
	v := value.NewBlock(kind)
	for _, assign := range typ.Block.Ast.Assignments {
		cv, diags := analyzer.ConstFold(assign.Value, ap)
		if len(diags) > 0 || cv.Kind == analyzer.ConstUnknown || cv.Partial {
			return value.Value{}, false
		}
		fv, ok := value.ValueFromConst(cv, nil)
		if !ok {
			return value.Value{}, false
		}
		key := value.NewString(assign.Name)
		var merged bool
		v, merged = v.MapWith(key, fv)
		if !merged {
			return value.Value{}, false
		}
	}
	return v, true
}

// buildRuntimeValueMap materializes one runtime Value per workflow node or trigger name.
func buildRuntimeValueMap(ap *analyzer.AnalyzedProgram, rw *workflow.ResolvedWorkflow) map[string]*value.Value {
	out := make(map[string]*value.Value)
	if ap == nil || rw == nil {
		return out
	}
	for _, name := range collectWorkflowNames(rw) {
		bn := resolveBlockName(rw, name)
		vv, ok := materializeBlock(ap, bn)
		if !ok {
			continue
		}
		cp := vv
		out[name] = &cp
	}
	return out
}

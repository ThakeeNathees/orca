package agentgraph

import (
	"testing"

	"github.com/thakee/orca/orca/compiler/analyzer"
	"github.com/thakee/orca/orca/compiler/ast"
	"github.com/thakee/orca/orca/compiler/lexer"
	"github.com/thakee/orca/orca/compiler/parser"
	"github.com/thakee/orca/orca/compiler/types"
	"github.com/thakee/orca/orca/compiler/workflow"
	"github.com/thakee/orca/orca/runtime/value"
)

func parseOrca(t *testing.T, src string) *ast.Program {
	t.Helper()
	l := lexer.New(src, "test.orca")
	p := parser.New(l)
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	return prog
}

// mustAnalyze parses and analyzes Orca source; fails if diagnostics are present.
func mustAnalyze(t *testing.T, src string) *analyzer.AnalyzedProgram {
	t.Helper()
	ap := analyzer.Analyze(parseOrca(t, src))
	if len(ap.Diagnostics) > 0 {
		t.Fatalf("analyzer diagnostics: %v", ap.Diagnostics)
	}
	return &ap
}

func mustWorkflowBlock(t *testing.T, ap *analyzer.AnalyzedProgram) *ast.BlockStatement {
	t.Helper()
	for _, stmt := range ap.Ast.Statements {
		b, ok := stmt.(*ast.BlockStatement)
		if ok && b.Kind == types.BlockKindWorkflow {
			return b
		}
	}
	t.Fatal("no workflow block in program")
	return nil
}

func TestResolveBlockName(t *testing.T) {
	tests := []struct {
		name string
		rw   *workflow.ResolvedWorkflow
		node string
		want string
	}{
		{name: "nil_rw_returns_node", rw: nil, node: "foo", want: "foo"},
		{name: "uses_direct_mapping", rw: &workflow.ResolvedWorkflow{NodeToBlockName: map[string]string{"x": "blk"}}, node: "x", want: "blk"},
		{name: "missing_mapping_falls_back", rw: &workflow.ResolvedWorkflow{NodeToBlockName: map[string]string{}}, node: "y", want: "y"},
		{name: "empty_mapping_value_falls_back", rw: &workflow.ResolvedWorkflow{NodeToBlockName: map[string]string{"z": ""}}, node: "z", want: "z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveBlockName(tt.rw, tt.node)
			if got != tt.want {
				t.Fatalf("resolveBlockName(...) = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCollectWorkflowNames(t *testing.T) {
	rw := workflow.ResolvedWorkflow{
		Nodes:    []string{"A", "B"},
		Triggers: []string{"cron_t", "hook_t"},
	}
	got := collectWorkflowNames(&rw)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4: %v", len(got), got)
	}
	// Duplicate across Nodes and Triggers should dedupe.
	rw2 := workflow.ResolvedWorkflow{
		Nodes:    []string{"dup"},
		Triggers: []string{"dup"},
	}
	got2 := collectWorkflowNames(&rw2)
	if len(got2) != 1 || got2[0] != "dup" {
		t.Fatalf("dedupe: got %v", got2)
	}
	if collectWorkflowNames(nil) != nil {
		t.Fatal("nil rw should return nil slice")
	}
}

func TestMaterializeBlock(t *testing.T) {
	const src = `webhook hooks_in {
  path = "/hooks/in"
  method = "POST"
}
agent A {
  model = gpt4
  persona = "p"
}
model gpt4 {
  provider = "openai"
  model_name = "gpt-4o"
}
workflow run {
  hooks_in -> A
}
`
	ap := mustAnalyze(t, src)

	tests := []struct {
		name    string
		block   string
		wantOk  bool
		check   func(t *testing.T, v value.Value)
		wantNil bool
	}{
		{
			name:   "webhook_hooks_in",
			block:  "hooks_in",
			wantOk: true,
			check: func(t *testing.T, v value.Value) {
				bk, ok := v.BlockKind()
				if !ok || bk != types.BlockKindWebhook {
					t.Fatalf("BlockKind = (%q,%v), want (webhook,true)", bk, ok)
				}
				p, ok := fieldString(t, v, "path")
				if !ok || p != "/hooks/in" {
					t.Fatalf("path = %q", p)
				}
				m, ok := fieldString(t, v, "method")
				if !ok || m != "POST" {
					t.Fatalf("method = %q", m)
				}
			},
		},
		{
			name:    "nil_program",
			block:   "hooks_in",
			wantOk:  false,
			wantNil: true,
		},
		{
			name:   "unknown_block",
			block:  "does_not_exist",
			wantOk: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var prog *analyzer.AnalyzedProgram
			if !tt.wantNil {
				prog = ap
			}
			v, ok := materializeBlock(prog, tt.block)
			if ok != tt.wantOk {
				t.Fatalf("materializeBlock ok = %v, want %v", ok, tt.wantOk)
			}
			if tt.check != nil {
				tt.check(t, v)
			}
		})
	}
}

func fieldString(t *testing.T, block value.Value, field string) (string, bool) {
	t.Helper()
	keys, vals, ok := block.KeysValues()
	if !ok {
		return "", false
	}
	for i := range keys {
		k, ko := keys[i].String()
		if ko && k == field {
			return vals[i].String()
		}
	}
	return "", false
}

func TestBuildRuntimeValueMap(t *testing.T) {
	const src = `webhook hooks_in {
  path = "/hooks/in"
}
agent A {
  model = gpt4
  persona = "p"
}
model gpt4 {
  provider = "openai"
  model_name = "gpt-4o"
}
workflow run {
  hooks_in -> A
}
`
	ap := mustAnalyze(t, src)
	wf := mustWorkflowBlock(t, ap)
	rw := workflow.ResolveV2(ap, wf)

	m := buildRuntimeValueMap(ap, &rw)
	if len(m) == 0 {
		t.Fatal("expected non-empty map")
	}
	hookV, ok := m["hooks_in"]
	if !ok || hookV == nil || !hookV.IsWebhookNode() {
		t.Fatal("expected webhook materialized under triggers name hooks_in")
	}
	agentV, ok := m["A"]
	if !ok || agentV == nil {
		t.Fatal("expected agent node A")
	}
	bk, ok := agentV.BlockKind()
	if !ok || bk != types.BlockKindAgent {
		t.Fatalf("A BlockKind = %q", bk)
	}
}

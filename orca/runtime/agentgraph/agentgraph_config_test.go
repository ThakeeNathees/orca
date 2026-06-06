package agentgraph

import (
	"testing"

	"github.com/thakee/orca/orca/compiler/ast"
	"github.com/thakee/orca/orca/compiler/types"
	"github.com/thakee/orca/orca/compiler/workflow"
	"github.com/thakee/orca/orca/runtime/db"
	"github.com/thakee/orca/orca/runtime/value"
)

// makeWorkflowBlockWithField builds a synthetic workflow BlockStatement with a
// single assignment. We construct the AST directly because the analyzer's
// schema currently rejects webhook_port as an unknown field on workflow, even
// though the runtime reads it through GetFieldExpression. Once the schema is
// updated, this helper can be replaced with mustAnalyze sources.
func makeWorkflowBlockWithField(field string, val ast.Expression) *ast.BlockStatement {
	return &ast.BlockStatement{
		BlockBody: ast.BlockBody{
			Kind: types.BlockKindWorkflow,
			Name: "run",
			Assignments: []*ast.Assignment{
				{Name: field, Value: val},
			},
		},
	}
}

// TestGetWebhookPort verifies the precedence: nil workflow falls back to the
// default, a workflow without webhook_port also falls back, a numeric
// webhook_port is read, and a non-numeric value falls back to the default.
func TestGetWebhookPort(t *testing.T) {
	tests := []struct {
		name  string
		block *ast.BlockStatement
		want  int
	}{
		{
			name:  "nil_workflow_returns_default",
			block: nil,
			want:  types.DefaultWebhookPort,
		},
		{
			name: "workflow_without_field_returns_default",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{Kind: types.BlockKindWorkflow, Name: "run"},
			},
			want: types.DefaultWebhookPort,
		},
		{
			name:  "numeric_field_overrides_default",
			block: makeWorkflowBlockWithField(types.WebhookPortField, &ast.NumberLiteral{Value: 9999}),
			want:  9999,
		},
		{
			name:  "string_field_falls_back_to_default",
			block: makeWorkflowBlockWithField(types.WebhookPortField, &ast.StringLiteral{Value: "not-a-number"}),
			want:  types.DefaultWebhookPort,
		},
		{
			name:  "unrelated_field_falls_back_to_default",
			block: makeWorkflowBlockWithField("unrelated", &ast.NumberLiteral{Value: 1234}),
			want:  types.DefaultWebhookPort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AgentGraphConfig{WorkflowBlock: tt.block}
			got := getWebhookPort(cfg)
			if got != tt.want {
				t.Fatalf("getWebhookPort = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestGetWebhookHandlers ensures only webhook-kind triggers surface as HTTP
// handlers, the normalized path comes from the materialized block, non-webhook
// triggers (e.g. cron) are skipped, and nil program returns nothing.
func TestGetWebhookHandlers(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		nilProgram bool
		wantPaths  []string
	}{
		{
			name: "webhook_trigger_produces_handler_with_normalized_path",
			src: `webhook hooks_in {
  path = "/hooks/in"
  method = "POST"
}
agent A { model = m persona = "p" }
model m { provider = "openai" model_name = "x" }
workflow run { hooks_in -> A }`,
			wantPaths: []string{"/hooks/in"},
		},
		{
			name: "cron_trigger_is_not_emitted_as_webhook_handler",
			src: `cron tick {
  schedule = "0 * * * *"
}
agent A { model = m persona = "p" }
model m { provider = "openai" model_name = "x" }
workflow run { tick -> A }`,
			wantPaths: nil,
		},
		{
			name:       "nil_program_returns_no_handlers",
			nilProgram: true,
			wantPaths:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ag := buildAgentGraphForTest(t, tt.src, tt.nilProgram)
			handlers := ag.getWebhookHandlers()
			if len(handlers) != len(tt.wantPaths) {
				t.Fatalf("handler count = %d, want %d", len(handlers), len(tt.wantPaths))
			}
			for i, want := range tt.wantPaths {
				if handlers[i].Endpoint != want {
					t.Fatalf("handler[%d].Endpoint = %q, want %q", i, handlers[i].Endpoint, want)
				}
				if handlers[i].Handler == nil {
					t.Fatalf("handler[%d].Handler is nil", i)
				}
			}
		})
	}
}

// buildAgentGraphForTest wires the same fields NewAgentGraph would set so
// methods that need prog/resolved/stack (getWebhookHandlers, LookupValue) work
// without StartMainLoop. nilProgram returns a bare AgentGraph for negative paths.
func buildAgentGraphForTest(t *testing.T, src string, nilProgram bool) *AgentGraph {
	t.Helper()
	if nilProgram {
		return &AgentGraph{db: db.NewInMemoryDB()}
	}
	ap := mustAnalyze(t, src)
	wf := mustWorkflowBlock(t, ap)
	rw := workflow.ResolveV2(ap, wf)
	vm := buildRuntimeValueMap(ap, &rw)
	return &AgentGraph{
		db:       db.NewInMemoryDB(),
		prog:     ap,
		resolved: rw,
		stack:    value.NewStack(nil, vm),
	}
}

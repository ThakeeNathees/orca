package agentgraph

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/thakee/orca/orca/compiler/workflow"
	"github.com/thakee/orca/orca/runtime/db"
	"github.com/thakee/orca/orca/runtime/value"
)

// mapFieldNumber reads a numeric field from a map-like Value (JSON object).
func mapFieldNumber(pl value.Value, key string) (float64, bool) {
	keys, vals, ok := pl.KeysValues()
	if !ok {
		return 0, false
	}
	for i := range keys {
		k, ko := keys[i].String()
		if ko && k == key {
			return vals[i].Number()
		}
	}
	return 0, false
}

// webhookProgramMinimal returns analyzed source with webhook trigger hooks_in and processing agent A.
func webhookProgramMinimal(t *testing.T) (*AgentGraph, *value.Value) {
	t.Helper()
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
	wf := mustWorkflowBlock(t, ap)
	rw := workflow.ResolveV2(ap, wf)
	vm := buildRuntimeValueMap(ap, &rw)
	v := vm["hooks_in"]
	if v == nil {
		t.Fatal("missing hooks_in value")
	}
	ag := &AgentGraph{
		ctx:    context.Background(),
		events: make(chan db.Event, 8),
	}
	return ag, v
}

func TestGetWebhookHandlerMissingRunID(t *testing.T) {
	ag, v := webhookProgramMinimal(t)
	wh := GetWebhookHandler(ag, "hooks_in", v)

	stream := make(chan any, 4)
	req := httptest.NewRequest(http.MethodPost, "/hooks/in", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	wh.Handler(rec, req, stream)
	close(stream)

	var sawErr bool
	for msg := range stream {
		m, ok := msg.(map[string]string)
		if ok && m["error"] != "" {
			sawErr = true
		}
	}
	if !sawErr {
		t.Fatal("expected error map on stream")
	}
}

func TestGetWebhookHandlerMethodMismatch(t *testing.T) {
	ag, v := webhookProgramMinimal(t)
	wh := GetWebhookHandler(ag, "hooks_in", v)

	stream := make(chan any, 4)
	req := httptest.NewRequest(http.MethodGet, "/hooks/in?run_id=x", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	wh.Handler(rec, req, stream)
	close(stream)

	var saw bool
	for msg := range stream {
		if m, ok := msg.(map[string]string); ok && strings.Contains(m["error"], "method") {
			saw = true
		}
	}
	if !saw {
		t.Fatal("expected method mismatch error")
	}
}

func TestGetWebhookHandlerInvalidJSON(t *testing.T) {
	ag, v := webhookProgramMinimal(t)
	wh := GetWebhookHandler(ag, "hooks_in", v)

	stream := make(chan any, 4)
	req := httptest.NewRequest(http.MethodPost, "/hooks/in?run_id=r", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()

	wh.Handler(rec, req, stream)
	close(stream)

	var saw bool
	for msg := range stream {
		if m, ok := msg.(map[string]string); ok && m["error"] != "" {
			saw = true
		}
	}
	if !saw {
		t.Fatal("expected JSON error on stream")
	}
}

func TestGetWebhookHandlerEnqueuesEvent(t *testing.T) {
	ag, v := webhookProgramMinimal(t)
	wh := GetWebhookHandler(ag, "hooks_in", v)

	stream := make(chan any, 4)
	req := httptest.NewRequest(http.MethodPost, "/hooks/in?run_id=run-9", strings.NewReader(`{"n":3}`))
	rec := httptest.NewRecorder()

	wh.Handler(rec, req, stream)

	select {
	case ev := <-ag.events:
		if ev.Kind != db.EventTypeWebhook || ev.RunID != "run-9" || ev.NodeID != "hooks_in" {
			t.Fatalf("event: %+v", ev)
		}
		if ev.WebhookData == nil {
			t.Fatal("nil WebhookData")
		}
		n, ok := mapFieldNumber(ev.WebhookData.Payload, "n")
		if !ok || n != 3 {
			t.Fatalf("payload n = (%v,%v)", n, ok)
		}
	default:
		t.Fatal("expected event on ag.events")
	}

	close(stream)
	for range stream {
	}
}

func TestGetWebhookHandlerIngressBehavior(t *testing.T) {
	tests := []struct {
		name        string
		setupAgent  func(*AgentGraph)
		wantErrText string
	}{
		{
			name: "returns_overloaded_error_when_event_queue_full",
			setupAgent: func(ag *AgentGraph) {
				for i := 0; i < cap(ag.events); i++ {
					ag.events <- *db.NewRunNodeEvent("busy-run", "n")
				}
			},
			wantErrText: "runtime overloaded",
		},
		{
			name: "returns_shutdown_error_when_context_done",
			setupAgent: func(ag *AgentGraph) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				ag.ctx = ctx
			},
			wantErrText: "runtime shutting down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ag, v := webhookProgramMinimal(t)
			if tt.setupAgent != nil {
				tt.setupAgent(ag)
			}
			wh := GetWebhookHandler(ag, "hooks_in", v)
			stream := make(chan any, 4)
			req := httptest.NewRequest(http.MethodPost, "/hooks/in?run_id=r", strings.NewReader(`{"ok":true}`))
			rec := httptest.NewRecorder()

			wh.Handler(rec, req, stream)
			select {
			case got := <-stream:
				msg, ok := got.(map[string]string)
				if !ok {
					t.Fatalf("unexpected response type: %T", got)
				}
				if !strings.Contains(msg["error"], tt.wantErrText) {
					t.Fatalf("error = %q, want contains %q", msg["error"], tt.wantErrText)
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatal("timed out waiting for ingress error response")
			}
		})
	}
}

func TestRespondWebhookMissingExecution(t *testing.T) {
	stream := make(chan any, 2)
	ev := db.NewNodeEventWebhook("r1", "hook", value.NewNil())
	ev.WebhookData.RespondCallback = func(inner func(chan any)) {
		inner(stream)
	}

	respondWebhookMissingExecution(ev, "execution not found for run_id")

	got := <-stream
	if got != "execution not found for run_id" {
		t.Fatalf("got %v", got)
	}
}

func TestHandleEventWebhookMissingExecution(t *testing.T) {
	stream := make(chan any, 2)
	ev := db.NewNodeEventWebhook("missing-run", "hook", value.NewNil())
	ev.WebhookData.RespondCallback = func(inner func(chan any)) {
		inner(stream)
	}

	ag := &AgentGraph{logger: nil}
	ag.handleEvent(ev, map[string]*Execution{})

	got := <-stream
	if got != "execution not found for run_id" {
		t.Fatalf("got %v", got)
	}
}

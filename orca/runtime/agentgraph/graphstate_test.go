package agentgraph

import (
	"strings"
	"testing"
	"time"

	"github.com/thakee/orca/orca/runtime/db"
	"github.com/thakee/orca/orca/runtime/value"
)

func TestStateOrIdle(t *testing.T) {
	m := map[string]NodeState{"x": NodeStateRunning}
	if stateOrIdle(m, "x") != NodeStateRunning {
		t.Fatal()
	}
	if stateOrIdle(m, "missing") != NodeStateIdle {
		t.Fatal()
	}
}

func TestTransitionReplayRunNodeThenListeningThenWebhook(t *testing.T) {
	st := map[string]NodeState{}
	data := map[string]value.Value{}
	errs := map[string]value.Value{}

	if err := transitionReplay(st, data, errs, db.NewRunNodeEvent("r", "n")); err != nil {
		t.Fatal(err)
	}
	if st["n"] != NodeStateRunning {
		t.Fatalf("after run_node: %v", st)
	}

	listen := &db.Event{Kind: db.EventTypeListening, RunID: "r", NodeID: "n"}
	if err := transitionReplay(st, data, errs, listen); err != nil {
		t.Fatal(err)
	}
	if st["n"] != NodeStateListening {
		t.Fatalf("after listening: %v", st)
	}

	payload := value.NewString("ok")
	wh := db.NewNodeEventWebhook("r", "n", payload)
	if err := transitionReplay(st, data, errs, wh); err != nil {
		t.Fatal(err)
	}
	if st["n"] != NodeStateWebhookDispatched {
		t.Fatalf("after webhook: %v", st)
	}
	s, ok := data["n"].String()
	if !ok || s != "ok" {
		t.Fatal(data["n"])
	}
}

func TestTransitionReplayErrors(t *testing.T) {
	tests := []struct {
		name string
		ev   *db.Event
	}{
		{
			name: "webhook_without_listening",
			ev:   db.NewNodeEventWebhook("r", "n", value.NewNil()),
		},
		{
			name: "listening_without_running",
			ev:   &db.Event{Kind: db.EventTypeListening, RunID: "r", NodeID: "n"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := map[string]NodeState{}
			data := map[string]value.Value{}
			errs := map[string]value.Value{}
			err := transitionReplay(st, data, errs, tt.ev)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}

	t.Run("nil_event", func(t *testing.T) {
		err := transitionReplay(map[string]NodeState{}, map[string]value.Value{}, map[string]value.Value{}, nil)
		if err == nil || !strings.Contains(err.Error(), "nil") {
			t.Fatal(err)
		}
	})

	t.Run("webhook_nil_payload_struct", func(t *testing.T) {
		ev := &db.Event{Kind: db.EventTypeWebhook, RunID: "r", NodeID: "n"}
		err := transitionReplay(map[string]NodeState{"n": NodeStateListening}, map[string]value.Value{}, map[string]value.Value{}, ev)
		if err == nil || !strings.Contains(err.Error(), "WebhookData") {
			t.Fatal(err)
		}
	})
}

func TestTransitionReplayDoneAndError(t *testing.T) {
	t.Run("done", func(t *testing.T) {
		st := map[string]NodeState{"n": NodeStateRunning}
		data := map[string]value.Value{}
		errs := map[string]value.Value{}
		ev := db.NewNodeEventDone("r", "n", value.NewNumber(7))
		if err := transitionReplay(st, data, errs, ev); err != nil {
			t.Fatal(err)
		}
		if st["n"] != NodeStateIdle {
			t.Fatal(st["n"])
		}
		n, _ := data["n"].Number()
		if n != 7 {
			t.Fatal(n)
		}
	})

	t.Run("error", func(t *testing.T) {
		st := map[string]NodeState{"n": NodeStateRunning}
		data := map[string]value.Value{}
		errs := map[string]value.Value{}
		ev := db.NewNodeEventError("r", "n", value.NewString("e"))
		if err := transitionReplay(st, data, errs, ev); err != nil {
			t.Fatal(err)
		}
		if st["n"] != NodeStateIdle {
			t.Fatal(st["n"])
		}
		s, _ := errs["n"].String()
		if s != "e" {
			t.Fatal(s)
		}
	})
}

func TestLoadGraphStateReplayOrder(t *testing.T) {
	d := db.NewInMemoryDB()
	const runID = "run1"

	_ = d.AddEvent(runID, db.NewRunNodeEvent(runID, "node"))
	_ = d.AddEvent(runID, &db.Event{Kind: db.EventTypeListening, RunID: runID, NodeID: "node"})
	_ = d.AddEvent(runID, db.NewNodeEventWebhook(runID, "node", value.NewBool(true)))

	gs, err := LoadGraphState(runID, d)
	if err != nil {
		t.Fatal(err)
	}
	st, _, _ := gs.Snapshot()
	if st["node"] != NodeStateWebhookDispatched {
		t.Fatalf("got %v", st)
	}
}

func TestDispatchWebhookQueuePolicy(t *testing.T) {
	tests := []struct {
		name                string
		initialState        NodeState
		inboxSize           int
		preloadInbox        bool
		wantResponse        string
		wantErrContains     string
		wantState           NodeState
		wantRunNodeEnqueued bool
		wantHasError        bool
		wantPersistedKinds  []db.EventKind
	}{
		{
			name:                "listening_and_capacity_enqueues_run_node",
			initialState:        NodeStateListening,
			inboxSize:           2,
			wantResponse:        "Acknowledged",
			wantState:           NodeStateWebhookDispatched,
			wantRunNodeEnqueued: true,
			wantHasError:        false,
			wantPersistedKinds:  []db.EventKind{db.EventTypeWebhook, db.EventTypeRunNode},
		},
		{
			name:                "listening_and_full_inbox_returns_busy_without_deadlock",
			initialState:        NodeStateListening,
			inboxSize:           1,
			preloadInbox:        true,
			wantResponse:        "Runtime busy, try again",
			wantErrContains:     "inbox full",
			wantState:           NodeStateWebhookDispatched,
			wantRunNodeEnqueued: false,
			wantHasError:        true,
			wantPersistedKinds:  []db.EventKind{db.EventTypeWebhook, db.EventTypeRunNode},
		},
		{
			name:                "not_listening_returns_rejection",
			initialState:        NodeStateIdle,
			inboxSize:           1,
			wantResponse:        "Webhook not listening",
			wantState:           NodeStateIdle,
			wantRunNodeEnqueued: false,
			wantHasError:        false,
			wantPersistedKinds:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := db.NewInMemoryDB()
			exec := &Execution{
				ag: &AgentGraph{
					db: database,
				},
				runID: "run-dispatch",
				graphState: &GraphState{
					nodeStates: map[string]NodeState{
						"hook": tt.initialState,
					},
					nodeData:   map[string]value.Value{},
					nodeErrors: map[string]value.Value{},
				},
				eventInbox: make(chan db.Event, tt.inboxSize),
			}
			if tt.preloadInbox {
				exec.eventInbox <- *db.NewRunNodeEvent(exec.runID, "already-queued")
			}

			resp := make(chan any, 1)
			ev := db.NewNodeEventWebhook(exec.runID, "hook", value.NewString("payload"))
			ev.WebhookData.RespondCallback = func(inner func(chan any)) {
				inner(resp)
			}

			start := time.Now()
			err := exec.graphState.DispatchWebhook(exec, ev)
			if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
				t.Fatalf("DispatchWebhook blocked too long: %s", elapsed)
			}

			if tt.wantErrContains == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErrContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("error = %v, want contains %q", err, tt.wantErrContains)
				}
			}

			select {
			case got := <-resp:
				if got != tt.wantResponse {
					t.Fatalf("response = %v, want %q", got, tt.wantResponse)
				}
			case <-time.After(300 * time.Millisecond):
				t.Fatal("timed out waiting for response callback")
			}

			states, _, errs := exec.graphState.Snapshot()
			if states["hook"] != tt.wantState {
				t.Fatalf("state = %q, want %q", states["hook"], tt.wantState)
			}
			_, hasErr := errs["hook"]
			if hasErr != tt.wantHasError {
				t.Fatalf("hasErr = %v, want %v", hasErr, tt.wantHasError)
			}

			runNodeQueued := false
			for len(exec.eventInbox) > 0 {
				got := <-exec.eventInbox
				if got.Kind == db.EventTypeRunNode && got.NodeID == "hook" {
					runNodeQueued = true
				}
			}
			if runNodeQueued != tt.wantRunNodeEnqueued {
				t.Fatalf("runNodeQueued = %v, want %v", runNodeQueued, tt.wantRunNodeEnqueued)
			}

			persisted := database.GetEvents(exec.runID)
			if len(persisted) != len(tt.wantPersistedKinds) {
				t.Fatalf("persisted event count = %d, want %d", len(persisted), len(tt.wantPersistedKinds))
			}
			for i, wantKind := range tt.wantPersistedKinds {
				if persisted[i].Kind != wantKind {
					t.Fatalf("persisted[%d].Kind = %q, want %q", i, persisted[i].Kind, wantKind)
				}
			}
		})
	}
}

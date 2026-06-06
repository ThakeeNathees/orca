package agentgraph

import (
	"strings"
	"testing"

	"github.com/thakee/orca/orca/runtime/db"
	"github.com/thakee/orca/orca/runtime/value"
)

// TestExecutionHandleEventDispatch verifies that Execution.handleEvent routes
// each EventKind into the correct GraphState mutation and surfaces RunID
// mismatches as errors. It avoids the run_node + listening + webhook
// orchestration paths covered by the existing event-loop tests, instead
// asserting the dispatch table itself: each kind triggers the right
// transition (or no-op) so adding a new EventKind without wiring it here
// surfaces immediately.
func TestExecutionHandleEventDispatch(t *testing.T) {
	const (
		runID  = "run-dispatch"
		nodeID = "n"
	)

	tests := []struct {
		name            string
		initialState    NodeState
		buildEvent      func() *db.Event
		wantErrContains string
		wantState       NodeState
		wantStreamCount int
		wantPersisted   []db.EventKind
	}{
		{
			name:         "listening_event_moves_running_to_listening",
			initialState: NodeStateRunning,
			buildEvent: func() *db.Event {
				return &db.Event{Kind: db.EventTypeListening, RunID: runID, NodeID: nodeID}
			},
			wantState:     NodeStateListening,
			wantPersisted: []db.EventKind{db.EventTypeListening},
		},
		{
			name:         "done_event_moves_running_to_idle",
			initialState: NodeStateRunning,
			buildEvent: func() *db.Event {
				return db.NewNodeEventDone(runID, nodeID, value.NewNumber(5))
			},
			wantState:     NodeStateIdle,
			wantPersisted: []db.EventKind{db.EventTypeDone},
		},
		{
			name:         "error_event_moves_running_to_idle_and_records_error",
			initialState: NodeStateRunning,
			buildEvent: func() *db.Event {
				return db.NewNodeEventError(runID, nodeID, value.NewString("boom"))
			},
			wantState:     NodeStateIdle,
			wantPersisted: []db.EventKind{db.EventTypeError},
		},
		{
			name:         "stream_event_is_forwarded_without_state_change_or_persist",
			initialState: NodeStateRunning,
			buildEvent: func() *db.Event {
				return db.NewNodeEventMsgStream(runID, nodeID, "token", "hi")
			},
			wantState:       NodeStateRunning,
			wantStreamCount: 1,
		},
		{
			name:         "run_id_mismatch_returns_error_without_state_change",
			initialState: NodeStateRunning,
			buildEvent: func() *db.Event {
				return &db.Event{Kind: db.EventTypeListening, RunID: "wrong-run", NodeID: nodeID}
			},
			wantErrContains: "run ID mismatch",
			wantState:       NodeStateRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := db.NewInMemoryDB()
			gs := &GraphState{
				nodeStates: map[string]NodeState{nodeID: tt.initialState},
				nodeData:   map[string]value.Value{},
				nodeErrors: map[string]value.Value{},
			}
			var streamed []*db.Event
			ag := &AgentGraph{
				db: database,
				streamHandler: func(ev *db.Event) {
					streamed = append(streamed, ev)
				},
			}
			exec := &Execution{ag: ag, runID: runID, graphState: gs}

			err := exec.handleEvent(tt.buildEvent())
			if tt.wantErrContains == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("err = %v, want contains %q", err, tt.wantErrContains)
				}
			}

			states, _, _ := gs.Snapshot()
			if states[nodeID] != tt.wantState {
				t.Fatalf("state = %q, want %q", states[nodeID], tt.wantState)
			}

			persisted := database.GetEvents(runID)
			if len(persisted) != len(tt.wantPersisted) {
				t.Fatalf("persisted count = %d, want %d", len(persisted), len(tt.wantPersisted))
			}
			for i, want := range tt.wantPersisted {
				if persisted[i].Kind != want {
					t.Fatalf("persisted[%d].Kind = %q, want %q", i, persisted[i].Kind, want)
				}
			}

			if len(streamed) != tt.wantStreamCount {
				t.Fatalf("stream forwards = %d, want %d", len(streamed), tt.wantStreamCount)
			}
		})
	}
}

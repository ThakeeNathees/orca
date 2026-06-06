package agentgraph

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/thakee/orca/orca/runtime/db"
	"github.com/thakee/orca/orca/runtime/value"
)

// failingDB wraps db.InMemoryDB to inject an AddEvent error for rollback tests.
// Reads delegate to the underlying in-memory DB so existing helpers keep working.
type failingDB struct {
	mu       sync.Mutex
	inner    *db.InMemoryDB
	failKind db.EventKind
	failErr  error
}

func newFailingDB(failKind db.EventKind) *failingDB {
	return &failingDB{
		inner:    db.NewInMemoryDB(),
		failKind: failKind,
		failErr:  errors.New("synthetic db failure"),
	}
}

func (f *failingDB) GetRunningExecutionIDs() []string { return f.inner.GetRunningExecutionIDs() }
func (f *failingDB) EnsureExecution(runID string) error {
	return f.inner.EnsureExecution(runID)
}
func (f *failingDB) CompleteExecution(runID string) error {
	return f.inner.CompleteExecution(runID)
}
func (f *failingDB) GetEvents(runID string) []*db.Event { return f.inner.GetEvents(runID) }
func (f *failingDB) AddEvent(runID string, ev *db.Event) error {
	f.mu.Lock()
	shouldFail := ev != nil && ev.Kind == f.failKind
	f.mu.Unlock()
	if shouldFail {
		return f.failErr
	}
	return f.inner.AddEvent(runID, ev)
}

// TestSetListening covers all branches of GraphState.SetListening: the success
// path from running -> listening, every invalid prior state, and the DB-write
// failure path which must roll the in-memory state back to prev.
func TestSetListening(t *testing.T) {
	const (
		runID  = "run-listen"
		nodeID = "n"
	)

	tests := []struct {
		name                string
		prev                NodeState
		event               *db.Event
		dbFailKind          db.EventKind
		wantErrContains     string
		wantState           NodeState
		wantPersistedEvents int
	}{
		{
			name:                "running_to_listening_success_persists_event",
			prev:                NodeStateRunning,
			event:               &db.Event{Kind: db.EventTypeListening, RunID: runID, NodeID: nodeID},
			wantState:           NodeStateListening,
			wantPersistedEvents: 1,
		},
		{
			name:            "idle_rejects_listening",
			prev:            NodeStateIdle,
			event:           &db.Event{Kind: db.EventTypeListening, RunID: runID, NodeID: nodeID},
			wantErrContains: "invalid state",
			wantState:       NodeStateIdle,
		},
		{
			name:            "listening_already_rejects",
			prev:            NodeStateListening,
			event:           &db.Event{Kind: db.EventTypeListening, RunID: runID, NodeID: nodeID},
			wantErrContains: "invalid state",
			wantState:       NodeStateListening,
		},
		{
			name:            "webhook_dispatched_rejects",
			prev:            NodeStateWebhookDispatched,
			event:           &db.Event{Kind: db.EventTypeListening, RunID: runID, NodeID: nodeID},
			wantErrContains: "invalid state",
			wantState:       NodeStateWebhookDispatched,
		},
		{
			name:            "nil_event_returns_error",
			prev:            NodeStateRunning,
			event:           nil,
			wantErrContains: "nil event",
			wantState:       NodeStateRunning,
		},
		{
			name:                "db_failure_rolls_state_back_to_running",
			prev:                NodeStateRunning,
			event:               &db.Event{Kind: db.EventTypeListening, RunID: runID, NodeID: nodeID},
			dbFailKind:          db.EventTypeListening,
			wantErrContains:     "synthetic db failure",
			wantState:           NodeStateRunning,
			wantPersistedEvents: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var database db.DB
			if tt.dbFailKind != "" {
				database = newFailingDB(tt.dbFailKind)
			} else {
				database = db.NewInMemoryDB()
			}
			gs := &GraphState{
				nodeStates: map[string]NodeState{nodeID: tt.prev},
				nodeData:   map[string]value.Value{},
				nodeErrors: map[string]value.Value{},
			}
			exec := &Execution{ag: &AgentGraph{db: database}, runID: runID, graphState: gs}

			err := gs.SetListening(exec, tt.event)
			checkErr(t, err, tt.wantErrContains)

			states, _, _ := gs.Snapshot()
			if states[nodeID] != tt.wantState {
				t.Fatalf("state = %q, want %q", states[nodeID], tt.wantState)
			}
			if got := len(database.GetEvents(runID)); got != tt.wantPersistedEvents {
				t.Fatalf("persisted = %d, want %d", got, tt.wantPersistedEvents)
			}
		})
	}
}

// TestSetNodeDone verifies the running -> idle transition: result merges into
// node data, the done event is persisted, invalid priors are rejected, and DB
// failure surfaces while leaving state unchanged on the failure path.
func TestSetNodeDone(t *testing.T) {
	const (
		runID  = "run-done"
		nodeID = "n"
	)

	mapResult, _ := value.NewMap().MapWith(value.NewString("k"), value.NewNumber(2))

	tests := []struct {
		name                string
		prev                NodeState
		priorData           value.Value
		event               *db.Event
		dbFailKind          db.EventKind
		wantErrContains     string
		wantState           NodeState
		wantPersistedEvents int
		checkData           func(t *testing.T, data value.Value)
	}{
		{
			name:                "running_to_idle_persists_and_replaces_scalar",
			prev:                NodeStateRunning,
			event:               db.NewNodeEventDone(runID, nodeID, value.NewNumber(7)),
			wantState:           NodeStateIdle,
			wantPersistedEvents: 1,
			checkData: func(t *testing.T, data value.Value) {
				n, ok := data.Number()
				if !ok || n != 7 {
					t.Fatalf("data = %+v, want 7", data)
				}
			},
		},
		{
			name:                "running_to_idle_merges_into_existing_map",
			prev:                NodeStateRunning,
			priorData:           mustMap(t, "a", value.NewNumber(1)),
			event:               db.NewNodeEventDone(runID, nodeID, mapResult),
			wantState:           NodeStateIdle,
			wantPersistedEvents: 1,
			checkData: func(t *testing.T, data value.Value) {
				keys, vals, ok := data.KeysValues()
				if !ok || len(keys) != 2 {
					t.Fatalf("expected merged map of 2 keys, got %+v", data)
				}
				gotKeys := map[string]float64{}
				for i := range keys {
					k, _ := keys[i].String()
					n, _ := vals[i].Number()
					gotKeys[k] = n
				}
				if gotKeys["a"] != 1 || gotKeys["k"] != 2 {
					t.Fatalf("merge mismatch: %v", gotKeys)
				}
			},
		},
		{
			name:            "idle_rejects",
			prev:            NodeStateIdle,
			event:           db.NewNodeEventDone(runID, nodeID, value.NewNil()),
			wantErrContains: "invalid state",
			wantState:       NodeStateIdle,
		},
		{
			name:            "listening_rejects",
			prev:            NodeStateListening,
			event:           db.NewNodeEventDone(runID, nodeID, value.NewNil()),
			wantErrContains: "invalid state",
			wantState:       NodeStateListening,
		},
		{
			name:            "nil_event_returns_error",
			prev:            NodeStateRunning,
			event:           nil,
			wantErrContains: "invalid event",
			wantState:       NodeStateRunning,
		},
		{
			name:            "missing_done_data_returns_error",
			prev:            NodeStateRunning,
			event:           &db.Event{Kind: db.EventTypeDone, RunID: runID, NodeID: nodeID},
			wantErrContains: "invalid event",
			wantState:       NodeStateRunning,
		},
		{
			name:            "db_failure_keeps_state_running_and_does_not_merge",
			prev:            NodeStateRunning,
			priorData:       value.NewNumber(1),
			event:           db.NewNodeEventDone(runID, nodeID, value.NewNumber(99)),
			dbFailKind:      db.EventTypeDone,
			wantErrContains: "synthetic db failure",
			wantState:       NodeStateRunning,
			checkData: func(t *testing.T, data value.Value) {
				n, ok := data.Number()
				if !ok || n != 1 {
					t.Fatalf("data = %+v, want 1 (unchanged)", data)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var database db.DB
			if tt.dbFailKind != "" {
				database = newFailingDB(tt.dbFailKind)
			} else {
				database = db.NewInMemoryDB()
			}
			gs := &GraphState{
				nodeStates: map[string]NodeState{nodeID: tt.prev},
				nodeData:   map[string]value.Value{},
				nodeErrors: map[string]value.Value{},
			}
			if tt.priorData.Kind() != value.KindNil {
				gs.nodeData[nodeID] = tt.priorData
			}
			exec := &Execution{ag: &AgentGraph{db: database}, runID: runID, graphState: gs}

			err := gs.SetNodeDone(exec, tt.event)
			checkErr(t, err, tt.wantErrContains)

			states, data, _ := gs.Snapshot()
			if states[nodeID] != tt.wantState {
				t.Fatalf("state = %q, want %q", states[nodeID], tt.wantState)
			}
			if got := len(database.GetEvents(runID)); got != tt.wantPersistedEvents {
				t.Fatalf("persisted = %d, want %d", got, tt.wantPersistedEvents)
			}
			if tt.checkData != nil {
				tt.checkData(t, data[nodeID])
			}
		})
	}
}

// TestSetNodeError verifies running -> idle on error: the node error map gets
// the new value, the error event is persisted, invalid priors are rejected,
// and DB failure leaves the state and error map unchanged.
func TestSetNodeError(t *testing.T) {
	const (
		runID  = "run-error"
		nodeID = "n"
	)

	tests := []struct {
		name                string
		prev                NodeState
		priorErr            value.Value
		event               *db.Event
		dbFailKind          db.EventKind
		wantErrContains     string
		wantState           NodeState
		wantPersistedEvents int
		wantStoredErr       string
	}{
		{
			name:                "running_records_error_and_returns_idle",
			prev:                NodeStateRunning,
			event:               db.NewNodeEventError(runID, nodeID, value.NewString("boom")),
			wantState:           NodeStateIdle,
			wantPersistedEvents: 1,
			wantStoredErr:       "boom",
		},
		{
			name:                "running_overwrites_prior_error_documents_known_limitation",
			prev:                NodeStateRunning,
			priorErr:            value.NewString("first"),
			event:               db.NewNodeEventError(runID, nodeID, value.NewString("second")),
			wantState:           NodeStateIdle,
			wantPersistedEvents: 1,
			wantStoredErr:       "second",
		},
		{
			name:            "idle_rejects",
			prev:            NodeStateIdle,
			event:           db.NewNodeEventError(runID, nodeID, value.NewString("e")),
			wantErrContains: "invalid state",
			wantState:       NodeStateIdle,
		},
		{
			name:            "listening_rejects",
			prev:            NodeStateListening,
			event:           db.NewNodeEventError(runID, nodeID, value.NewString("e")),
			wantErrContains: "invalid state",
			wantState:       NodeStateListening,
		},
		{
			name:            "nil_event_returns_error",
			prev:            NodeStateRunning,
			event:           nil,
			wantErrContains: "invalid event",
			wantState:       NodeStateRunning,
		},
		{
			name:            "missing_error_data_returns_error",
			prev:            NodeStateRunning,
			event:           &db.Event{Kind: db.EventTypeError, RunID: runID, NodeID: nodeID},
			wantErrContains: "invalid event",
			wantState:       NodeStateRunning,
		},
		{
			name:            "db_failure_keeps_state_running_and_does_not_record_error",
			prev:            NodeStateRunning,
			event:           db.NewNodeEventError(runID, nodeID, value.NewString("ignored")),
			dbFailKind:      db.EventTypeError,
			wantErrContains: "synthetic db failure",
			wantState:       NodeStateRunning,
			wantStoredErr:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var database db.DB
			if tt.dbFailKind != "" {
				database = newFailingDB(tt.dbFailKind)
			} else {
				database = db.NewInMemoryDB()
			}
			gs := &GraphState{
				nodeStates: map[string]NodeState{nodeID: tt.prev},
				nodeData:   map[string]value.Value{},
				nodeErrors: map[string]value.Value{},
			}
			if tt.priorErr.Kind() != value.KindNil {
				gs.nodeErrors[nodeID] = tt.priorErr
			}
			exec := &Execution{ag: &AgentGraph{db: database}, runID: runID, graphState: gs}

			err := gs.SetNodeError(exec, tt.event)
			checkErr(t, err, tt.wantErrContains)

			states, _, errs := gs.Snapshot()
			if states[nodeID] != tt.wantState {
				t.Fatalf("state = %q, want %q", states[nodeID], tt.wantState)
			}
			if got := len(database.GetEvents(runID)); got != tt.wantPersistedEvents {
				t.Fatalf("persisted = %d, want %d", got, tt.wantPersistedEvents)
			}
			if tt.wantStoredErr != "" {
				s, ok := errs[nodeID].String()
				if !ok || s != tt.wantStoredErr {
					t.Fatalf("stored err = (%q,%v), want %q", s, ok, tt.wantStoredErr)
				}
			}
		})
	}
}

// checkErr asserts err presence/absence and substring match. Empty wantSub means no error expected.
func checkErr(t *testing.T, err error, wantSub string) {
	t.Helper()
	if wantSub == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("want error containing %q, got nil", wantSub)
	}
	if !strings.Contains(err.Error(), wantSub) {
		t.Fatalf("err = %q, want contains %q", err.Error(), wantSub)
	}
}

// mustMap builds a single-key map for test setup; failing here means the test rig is wrong.
func mustMap(t *testing.T, key string, v value.Value) value.Value {
	t.Helper()
	m, ok := value.NewMap().MapWith(value.NewString(key), v)
	if !ok {
		t.Fatalf("MapWith failed for key %q", key)
	}
	return m
}

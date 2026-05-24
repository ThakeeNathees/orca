package agentgraph

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/thakee/orca/orca/runtime/db"
	"github.com/thakee/orca/orca/runtime/value"
)

type eventLoopTestDB struct {
	mu         sync.Mutex
	runningIDs []string
	completed  []string
	events     map[string][]*db.Event
}

func newEventLoopTestDB(runningIDs []string) *eventLoopTestDB {
	return &eventLoopTestDB{
		runningIDs: runningIDs,
		events:     make(map[string][]*db.Event),
	}
}

func (d *eventLoopTestDB) GetRunningExecutionIDs() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, len(d.runningIDs))
	copy(out, d.runningIDs)
	return out
}

func (d *eventLoopTestDB) AddEvent(runID string, event *db.Event) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.events[runID] = append(d.events[runID], event)
	return nil
}

func (d *eventLoopTestDB) EnsureExecution(runID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, id := range d.runningIDs {
		if id == runID {
			return nil
		}
	}
	d.runningIDs = append(d.runningIDs, runID)
	return nil
}

func (d *eventLoopTestDB) CompleteExecution(runID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.completed = append(d.completed, runID)
	filtered := make([]string, 0, len(d.runningIDs))
	for _, id := range d.runningIDs {
		if id != runID {
			filtered = append(filtered, id)
		}
	}
	d.runningIDs = filtered
	return nil
}

func (d *eventLoopTestDB) GetEvents(runID string) []*db.Event {
	d.mu.Lock()
	defer d.mu.Unlock()
	evs := d.events[runID]
	out := make([]*db.Event, len(evs))
	copy(out, evs)
	return out
}

func TestHandleEventRoutesKnownExecutionToInbox(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "known_run_id_routes_to_execution_inbox_without_direct_state_mutation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runID := "run-1"
			nodeID := "hooks_in"
			d := newEventLoopTestDB(nil)

			exec := &Execution{
				ag: &AgentGraph{
					db: d,
				},
				runID: runID,
				graphState: &GraphState{
					nodeStates: map[string]NodeState{nodeID: NodeStateListening},
					nodeData:   map[string]value.Value{},
					nodeErrors: map[string]value.Value{},
				},
				eventInbox: make(chan db.Event, 4),
			}

			ev := db.NewNodeEventWebhook(runID, nodeID, value.NewString("payload"))
			ag := &AgentGraph{}
			ag.handleEvent(ev, map[string]*Execution{runID: exec})

			select {
			case got := <-exec.eventInbox:
				if got.Kind != db.EventTypeWebhook {
					t.Fatalf("got queued kind %q, want %q", got.Kind, db.EventTypeWebhook)
				}
			default:
				t.Fatal("expected event to be queued into execution inbox")
			}

			states, _, _ := exec.graphState.Snapshot()
			if states[nodeID] != NodeStateListening {
				t.Fatalf("node state = %q, want %q", states[nodeID], NodeStateListening)
			}
			if len(d.GetEvents(runID)) != 0 {
				t.Fatalf("unexpected persisted events: %d", len(d.GetEvents(runID)))
			}
		})
	}
}

func TestHandleEventMissingExecutionCreatesExecutionAndResponds(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "missing_run_id_creates_execution_and_responds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := make(chan any, 1)
			ev := db.NewNodeEventWebhook("missing-run", "hook", value.NewNil())
			ev.WebhookData.RespondCallback = func(inner func(chan any)) {
				inner(stream)
			}

			d := newEventLoopTestDB(nil)
			ag := &AgentGraph{
				db: d,
			}
			ag.handleEvent(ev, map[string]*Execution{})

			select {
			case got := <-stream:
				// Fresh executions start idle in phase-1. The important invariant is no missing-execution path.
				if got == "execution not found for run_id" || got == "failed to load execution" {
					t.Fatalf("got %v", got)
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatal("timed out waiting for webhook response")
			}
			ids := d.GetRunningExecutionIDs()
			if len(ids) != 1 || ids[0] != "missing-run" {
				t.Fatalf("running IDs = %v", ids)
			}
		})
	}
}

func TestExecutionLoopProcessesWebhookAndEnqueuesRunNode(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "execution_loop_handles_webhook_then_processes_run_node"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runID := "run-2"
			nodeID := "hooks_in"
			d := newEventLoopTestDB(nil)
			exec := &Execution{
				ag: &AgentGraph{
					db: d,
				},
				runID: runID,
				graphState: &GraphState{
					nodeStates: map[string]NodeState{nodeID: NodeStateListening},
					nodeData:   map[string]value.Value{},
					nodeErrors: map[string]value.Value{},
				},
				eventInbox: make(chan db.Event, 4),
			}

			done := make(chan struct{})
			go func() {
				defer close(done)
				exec.Execute()
			}()
			defer func() {
				close(exec.eventInbox)
				<-done
			}()

			callbackCh := make(chan any, 1)
			webhookEvent := db.NewNodeEventWebhook(runID, nodeID, value.NewString("payload"))
			webhookEvent.WebhookData.RespondCallback = func(inner func(chan any)) {
				inner(callbackCh)
			}

			ag := &AgentGraph{}
			ag.handleEvent(webhookEvent, map[string]*Execution{runID: exec})

			select {
			case got := <-callbackCh:
				if got != "Acknowledged" {
					t.Fatalf("callback = %v, want Acknowledged", got)
				}
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for webhook callback")
			}

			waitForTestCondition(t, time.Second, func() bool {
				states, _, _ := exec.graphState.Snapshot()
				return states[nodeID] == NodeStateRunning
			})

			waitForTestCondition(t, time.Second, func() bool {
				evs := d.GetEvents(runID)
				if len(evs) < 2 {
					return false
				}
				return evs[0].Kind == db.EventTypeWebhook && evs[1].Kind == db.EventTypeRunNode
			})
		})
	}
}

func waitForTestCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func TestRouteEventToExecutionContextAware(t *testing.T) {
	tests := []struct {
		name           string
		setupCtx       func() context.Context
		preloadInbox   bool
		wantErrIs      error
		wantInboxCount int
	}{
		{
			name: "routes_when_context_nil",
			setupCtx: func() context.Context {
				return nil
			},
			wantErrIs:      nil,
			wantInboxCount: 1,
		},
		{
			name: "does_not_route_when_context_done",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErrIs:      errRouteExecutionShuttingDown,
			wantInboxCount: 0,
		},
		{
			name: "returns_backpressure_when_inbox_full",
			setupCtx: func() context.Context {
				return context.Background()
			},
			preloadInbox:   true,
			wantErrIs:      errRouteExecutionBackpressure,
			wantInboxCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &Execution{eventInbox: make(chan db.Event, 1)}
			if tt.preloadInbox {
				exec.eventInbox <- *db.NewRunNodeEvent("run-1", "preloaded")
			}
			ag := &AgentGraph{ctx: tt.setupCtx()}
			ev := db.NewRunNodeEvent("run-1", "n1")

			err := ag.routeEventToExecution(ev, exec)
			if tt.wantErrIs == nil && err != nil {
				t.Fatalf("routeEventToExecution() err = %v", err)
			}
			if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Fatalf("routeEventToExecution() err = %v, want %v", err, tt.wantErrIs)
			}
			if got := len(exec.eventInbox); got != tt.wantInboxCount {
				t.Fatalf("inbox length = %d, want %d", got, tt.wantInboxCount)
			}
		})
	}
}

func TestHandleEventBackpressureIsolation(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "saturated_run_does_not_block_unrelated_run"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newEventLoopTestDB(nil)
			execA := &Execution{
				ag:         &AgentGraph{db: d},
				runID:      "run-a",
				graphState: &GraphState{nodeStates: map[string]NodeState{}, nodeData: map[string]value.Value{}, nodeErrors: map[string]value.Value{}},
				eventInbox: make(chan db.Event, 1),
			}
			execA.eventInbox <- *db.NewRunNodeEvent("run-a", "preloaded")
			execB := &Execution{
				ag:         &AgentGraph{db: d},
				runID:      "run-b",
				graphState: &GraphState{nodeStates: map[string]NodeState{}, nodeData: map[string]value.Value{}, nodeErrors: map[string]value.Value{}},
				eventInbox: make(chan db.Event, 1),
			}

			ag := &AgentGraph{ctx: context.Background()}
			executionStates := map[string]*Execution{
				"run-a": execA,
				"run-b": execB,
			}

			ag.handleEvent(db.NewRunNodeEvent("run-a", "n1"), executionStates)
			ag.handleEvent(db.NewRunNodeEvent("run-b", "n2"), executionStates)

			select {
			case got := <-execB.eventInbox:
				if got.NodeID != "n2" {
					t.Fatalf("got node %q, want n2", got.NodeID)
				}
			default:
				t.Fatal("expected unrelated run event to be routed")
			}
		})
	}
}

func TestHandleEventShutdownRespondsDeterministically(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "webhook_route_failure_returns_shutdown_message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			d := newEventLoopTestDB(nil)
			exec := &Execution{
				ag:         &AgentGraph{db: d},
				runID:      "run-shutdown",
				graphState: &GraphState{nodeStates: map[string]NodeState{}, nodeData: map[string]value.Value{}, nodeErrors: map[string]value.Value{}},
				eventInbox: make(chan db.Event, 1),
			}
			stream := make(chan any, 1)
			ev := db.NewNodeEventWebhook("run-shutdown", "hook", value.NewNil())
			ev.WebhookData.RespondCallback = func(inner func(chan any)) { inner(stream) }

			ag := &AgentGraph{ctx: ctx}
			ag.handleEvent(ev, map[string]*Execution{"run-shutdown": exec})

			select {
			case got := <-stream:
				if got != "runtime shutting down" {
					t.Fatalf("got %v", got)
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatal("timed out waiting for shutdown response")
			}
		})
	}
}

func TestLoadExecutionsReloadsAndContinuesProcessing(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "reload_from_db_then_process_new_webhook"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runID := "run-reload"
			nodeID := "hooks_in"
			d := newEventLoopTestDB([]string{runID})
			if err := d.AddEvent(runID, db.NewRunNodeEvent(runID, nodeID)); err != nil {
				t.Fatal(err)
			}
			if err := d.AddEvent(runID, &db.Event{
				Kind:   db.EventTypeListening,
				RunID:  runID,
				NodeID: nodeID,
			}); err != nil {
				t.Fatal(err)
			}

			ag := &AgentGraph{
				db:             d,
				eventsBuffSize: 8,
			}
			executionStates := loadExecutions(ag)
			exec, ok := executionStates[runID]
			if !ok {
				t.Fatalf("expected execution %q to load", runID)
			}

			startExecutionLoops(executionStates)
			defer stopExecutionLoops(executionStates)

			stream := make(chan any, 1)
			webhookEvent := db.NewNodeEventWebhook(runID, nodeID, value.NewString("v"))
			webhookEvent.WebhookData.RespondCallback = func(inner func(chan any)) {
				inner(stream)
			}
			ag.handleEvent(webhookEvent, executionStates)

			select {
			case got := <-stream:
				if got != "Acknowledged" {
					t.Fatalf("callback = %v, want Acknowledged", got)
				}
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for webhook callback")
			}

			waitForTestCondition(t, time.Second, func() bool {
				states, _, _ := exec.graphState.Snapshot()
				return states[nodeID] == NodeStateRunning
			})
		})
	}
}

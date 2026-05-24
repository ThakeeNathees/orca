package agentgraph

import (
	"testing"
	"time"

	"github.com/thakee/orca/orca/runtime/db"
	"github.com/thakee/orca/orca/runtime/value"
)

func TestExecutionRecoveryReschedulesPendingNodes(t *testing.T) {
	tests := []struct {
		name               string
		seedEvents         func(d *db.InMemoryDB, runID string) error
		wantRecoveredState NodeState
		wantQueuedNodeID   string
		wantPersistedKinds []db.EventKind
	}{
		{
			name: "recovers_running_node_from_persisted_run_event",
			seedEvents: func(d *db.InMemoryDB, runID string) error {
				if err := d.AddEvent(runID, db.NewRunNodeEvent(runID, "node")); err != nil {
					return err
				}
				return nil
			},
			wantRecoveredState: NodeStateRunning,
			wantQueuedNodeID:   "node",
			wantPersistedKinds: []db.EventKind{db.EventTypeRunNode},
		},
		{
			name: "recovers_webhook_dispatched_by_persisting_and_rescheduling_run_event",
			seedEvents: func(d *db.InMemoryDB, runID string) error {
				if err := d.AddEvent(runID, db.NewRunNodeEvent(runID, "node")); err != nil {
					return err
				}
				if err := d.AddEvent(runID, &db.Event{Kind: db.EventTypeListening, RunID: runID, NodeID: "node"}); err != nil {
					return err
				}
				if err := d.AddEvent(runID, db.NewNodeEventWebhook(runID, "node", value.NewString("v"))); err != nil {
					return err
				}
				return nil
			},
			wantRecoveredState: NodeStateWebhookDispatched,
			wantQueuedNodeID:   "node",
			wantPersistedKinds: []db.EventKind{db.EventTypeRunNode, db.EventTypeListening, db.EventTypeWebhook, db.EventTypeRunNode},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := db.NewInMemoryDB()
			runID := "run-recover"
			if err := d.EnsureExecution(runID); err != nil {
				t.Fatal(err)
			}
			if err := tt.seedEvents(d, runID); err != nil {
				t.Fatal(err)
			}

			ag := &AgentGraph{
				db:             d,
				eventsBuffSize: 8,
			}
			executions := loadExecutions(ag)
			exec, ok := executions[runID]
			if !ok {
				t.Fatalf("expected execution %q", runID)
			}

			exec.recoverPendingNodes()

			states, _, _ := exec.graphState.Snapshot()
			if states["node"] != tt.wantRecoveredState {
				t.Fatalf("state = %q, want %q", states["node"], tt.wantRecoveredState)
			}

			waitForRecoveryCondition(t, time.Second, func() bool {
				return len(exec.eventInbox) > 0
			})
			queued := <-exec.eventInbox
			if queued.Kind != db.EventTypeRunNode || queued.NodeID != tt.wantQueuedNodeID {
				t.Fatalf("queued event = %+v, want run_node for %q", queued, tt.wantQueuedNodeID)
			}

			persisted := d.GetEvents(runID)
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

func waitForRecoveryCondition(t *testing.T, timeout time.Duration, cond func() bool) {
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

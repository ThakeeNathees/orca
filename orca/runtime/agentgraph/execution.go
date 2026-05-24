package agentgraph

import (
	"fmt"

	"github.com/thakee/orca/orca/runtime/db"
)

// An Execution is the running instance of an agent graph that contains local values and run id, etc.
// Each trigger will spawn a new execution and the execution will be terminated when the trigger is completed.
type Execution struct {
	ag         *AgentGraph
	runID      string
	graphState *GraphState
	eventInbox chan db.Event

	recoveredNodeStates map[string]NodeState
}

func loadExecutions(ag *AgentGraph) map[string]*Execution {
	executions := make(map[string]*Execution)
	for _, runningExecID := range ag.db.GetRunningExecutionIDs() {
		execution, err := LoadExecution(ag, runningExecID)
		if err != nil {
			if ag.logger != nil {
				ag.logger.Printf("failed to load execution for run ID %q: %v", runningExecID, err)
			}
			continue
		}
		executions[runningExecID] = execution
	}
	return executions
}

// If a checkpoint exists for the run ID, load it and return the execution, otherwise create a new execution.
func LoadExecution(ag *AgentGraph, runID string) (*Execution, error) {
	eventInbox := make(chan db.Event, max(1, ag.eventsBuffSize))
	gs, err := LoadGraphState(runID, ag.db)
	if err != nil {
		return nil, err
	}
	e := &Execution{
		ag:         ag,
		runID:      runID,
		graphState: gs,
		eventInbox: eventInbox,
	}
	states, _, _ := gs.Snapshot()
	e.recoveredNodeStates = states
	return e, nil
}

func (e *Execution) Execute() {
	for event := range e.eventInbox {
		if err := e.handleEvent(&event); err != nil && e.ag != nil && e.ag.logger != nil {
			e.ag.logger.Printf("error handling event: %s, event: %+v", err, event)
		}
	}
}

// RunNode executes the given workflow node. Placeholder until the node runtime is wired.
func (e *Execution) RunNode(nodeID string) {
	_ = nodeID
	_ = e
}

// recoverPendingNodes re-schedules nodes that were mid-flight at process restart.
func (e *Execution) recoverPendingNodes() {
	if e == nil || e.ag == nil || e.ag.db == nil {
		return
	}
	for nodeID, nodeState := range e.recoveredNodeStates {
		switch nodeState {
		case NodeStateRunning:
			runEv := db.NewRunNodeEvent(e.runID, nodeID)
			runEv.Persisted = true
			if err := enqueueRunNodeEvent(e, runEv); err != nil && e.ag.logger != nil {
				e.ag.logger.Printf("failed to recover running node %q for run %q: %v", nodeID, e.runID, err)
			}
		case NodeStateWebhookDispatched:
			runEv := db.NewRunNodeEvent(e.runID, nodeID)
			if err := e.ag.db.AddEvent(e.runID, runEv); err != nil {
				if e.ag.logger != nil {
					e.ag.logger.Printf("failed to persist recovered run_node for %q/%q: %v", e.runID, nodeID, err)
				}
				continue
			}
			runEv.Persisted = true
			if err := enqueueRunNodeEvent(e, runEv); err != nil && e.ag.logger != nil {
				e.ag.logger.Printf("failed to enqueue recovered run_node for %q/%q: %v", e.runID, nodeID, err)
			}
		}
	}
	e.recoveredNodeStates = nil
}

func (e *Execution) handleEvent(event *db.Event) error {
	// Assert event.RunID == e.runID
	if event.RunID != e.runID {
		return fmt.Errorf("event run ID mismatch, expected: %s, got: %s", e.runID, event.RunID)
	}

	// Handle the event.
	switch event.Kind {

	case db.EventTypeRunNode:
		return e.graphState.RunNode(e, event)

	case db.EventTypeListening:
		return e.graphState.SetListening(e, event)

	// All the triggering events are handled by the agent graph's listen loop.
	// This webhook is part of the execution, example: Human in the loop.
	case db.EventTypeWebhook:
		return e.graphState.DispatchWebhook(e, event)

	case db.EventTypeMsgStream:
		e.ag.handleStreamEvent(event)

	case db.EventTypeDone:
		return e.graphState.SetNodeDone(e, event)

	case db.EventTypeError:
		return e.graphState.SetNodeError(e, event)
		// TODO: add notification to user.
	}

	return nil
}

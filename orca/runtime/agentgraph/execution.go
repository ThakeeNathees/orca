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
}

func loadExecutions(ag *AgentGraph) map[string]*Execution {
	executions := make(map[string]*Execution)
	for _, runningExecID := range ag.db.GetRunningExecutionIDs() {
		execution, err := LoadExecution(ag, runningExecID)
		if err != nil {
			continue // TODO: Log the error or something.
		}
		executions[runningExecID] = execution
	}
	return executions
}

// If a checkpoint exists for the run ID, load it and return the execution, otherwise create a new execution.
func LoadExecution(ag *AgentGraph, runID string) (*Execution, error) {
	eventInbox := make(chan db.Event, max(1, ag.eventsBuffSize))
	e := &Execution{
		ag:         ag,
		runID:      runID,
		graphState: LoadGraphState(runID, ag.db),
		eventInbox: eventInbox,
	}
	return e, nil
}

func (e *Execution) Execute() {
	for event := range e.eventInbox {
		e.handleEvent(&event)
	}
}

func (e *Execution) handleEvent(event *db.Event) error {
	// Assert event.RunID == e.runID
	if event.RunID != e.runID {
		return fmt.Errorf("event run ID mismatch, expected: %s, got: %s", e.runID, event.RunID)
	}

	// Handle the event.
	switch event.Kind {

	case db.EventTypeRunNode:
		e.graphState.RunNode(e, event)

	case db.EventTypeListening:
		e.graphState.SetListening(e, event)

	// All the triggering events are handled by the agent graph's listen loop.
	// This webhook is part of the execution, example: Human in the loop.
	case db.EventTypeWebhook:
		e.graphState.DispatchWebhook(e, event)

	case db.EventTypeMsgStream:
		e.ag.handleStreamEvent(event)

	case db.EventTypeDone:
		e.graphState.SetNodeDone(e, event)

	case db.EventTypeError:
		e.graphState.SetNodeError(e, event)
		// TODO: add notification to user.
	}

	return nil
}

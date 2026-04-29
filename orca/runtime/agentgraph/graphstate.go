package agentgraph

import (
	"sync"

	"github.com/thakee/orca/orca/runtime/db"
	"github.com/thakee/orca/orca/runtime/value"
)

type NodeState string

const (
	NodeStateIdle    NodeState = "idle"    // The node is idle and waiting for a webhook.
	NodeStateRunning NodeState = "running" // The node is running and executing.

	// Only applicable for webhook nodes (Human in the loop)
	//
	// State transformation:
	// idle -> running (started) -> listening -> webhook_dispatched -> running -> idle
	NodeStateListening         NodeState = "listening"          // The node is listening for a webhook.
	NodeStateWebhookDispatched NodeState = "webhook_dispatched" // The node has dispatched a webhook and is waiting for a response.
)

// This is strictly not mutable by any executions, only mutable when
// the graph is loaded from the database and replayed.
// Or after a node is done and the vlaue is merged into the graph state.
type GraphState struct {
	mu         sync.Mutex
	nodeStates map[string]NodeState
	nodeData   map[string]value.Value

	// TODO: Errors should be list of values (we accumilate all the errors during the execution)
	nodeErrors map[string]value.Value
}

func LoadGraphState(runID string, db db.DB) *GraphState {

	graphState := &GraphState{
		nodeStates: make(map[string]NodeState),
		nodeData:   make(map[string]value.Value),
		nodeErrors: make(map[string]value.Value),
	}

	if db != nil {
		for _, event := range db.GetEvents(runID) {
			graphState.replay(event)
		}
	}

	return graphState
}

func (gs *GraphState) replay(event *db.Event) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	// All states are idle at start.
	for nodeID := range gs.nodeStates {
		gs.nodeStates[nodeID] = NodeStateIdle
	}

	switch event.Kind {
	case db.EventTypeWebhook:
		// If a webhook event is received, the node must be listening.
		if gs.nodeStates[event.NodeID] != NodeStateListening {
			panic("Webhook event: node not listening or node state missing")
		}
		gs.nodeData[event.NodeID] = event.WebhookData.Payload
		gs.nodeStates[event.NodeID] = NodeStateWebhookDispatched

	// These events are not part of the graph state, just streaming tokens, progress, etc.
	// mostly for UI
	case db.EventTypeMsgStream:
		break

	case db.EventTypeListening:
		if gs.nodeStates[event.NodeID] != NodeStateRunning {
			panic("Listening event: node not running or node state missing")
		}
		gs.nodeStates[event.NodeID] = NodeStateListening

	case db.EventTypeDone:
		if gs.nodeStates[event.NodeID] != NodeStateRunning {
			panic("Done event: node not running or node state missing")
		}
		gs.nodeData[event.NodeID] = value.Merge(gs.nodeData[event.NodeID], event.DoneData.Result)
		gs.nodeStates[event.NodeID] = NodeStateIdle

	case db.EventTypeError:
		if gs.nodeStates[event.NodeID] != NodeStateRunning {
			panic("Error event: node not running or node state missing")
		}
		gs.nodeErrors[event.NodeID] = event.ErrorData.Error
		gs.nodeStates[event.NodeID] = NodeStateIdle
	}
}

func (gs *GraphState) RunNode(e *Execution, event *db.Event) {
	gs.mu.Lock()

	// The node must be idle or webhook dispatched.
	if gs.nodeStates[event.NodeID] != NodeStateIdle && gs.nodeStates[event.NodeID] != NodeStateWebhookDispatched {
		panic("Run node event: node not idle or webhook dispatched or node state missing")
	}

	// 1. Update State to running.
	gs.nodeStates[event.NodeID] = NodeStateRunning
	// 2. Save the event to db.
	e.ag.db.AddEvent(event.RunID, event)

	// Mutex no longer needed (the bellow function may need mutex so we need to unlock it)
	gs.mu.Unlock()

	// 3. Start the coroutine to run the node.
	go e.RunNode(event.NodeID)
}

func (gs *GraphState) SetListening(e *Execution, event *db.Event) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if gs.nodeStates[event.NodeID] != NodeStateRunning {
		panic("Listening event: node not running or node state missing")
	}

	// 1. Update State to listening.
	gs.nodeStates[event.NodeID] = NodeStateListening
	// 2. Save the event to db.
	e.ag.db.AddEvent(event.RunID, event)
}

func (gs *GraphState) DispatchWebhook(e *Execution, event *db.Event) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	// Will be set to "Webhook not listening" if the node is not listening.
	webhookResponseMsg := ""
	if gs.nodeStates[event.NodeID] == NodeStateListening {
		webhookResponseMsg = "Acknowledged"
	} else if gs.nodeStates[event.NodeID] == NodeStateWebhookDispatched {
		webhookResponseMsg = "Webhook already dispatched"
	} else {
		webhookResponseMsg = "Webhook not listening"
	}

	defer func() {
		if event.WebhookData.RespondCallback != nil {
			// TODO: Respond proper json object with 200, or 400, etc.
			event.WebhookData.RespondCallback(func(r chan any) {
				r <- webhookResponseMsg
				close(r)
			})
		}
	}()

	if gs.nodeStates[event.NodeID] == NodeStateListening {
		// 1. Set the payload to the node data.
		gs.nodeData[event.NodeID] = event.WebhookData.Payload
		// 2. Update State to dispatched.
		gs.nodeStates[event.NodeID] = NodeStateWebhookDispatched
		// 3. Save the event to db.
		e.ag.db.AddEvent(event.RunID, event)
		// 4. Enqueue the node for running.
		e.eventInbox <- *NewRunNodeEvent(event.RunID, event.NodeID)
	}

}

func (gs *GraphState) SetNodeDone(e *Execution, event *db.Event) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if gs.nodeStates[event.NodeID] != NodeStateRunning {
		panic("Done event: node not running or node state missing")
	}

	// 1. Merge the result into the node data.
	gs.nodeData[event.NodeID] = value.Merge(gs.nodeData[event.NodeID], event.DoneData.Result)

	// 2. Update State to idle.
	gs.nodeStates[event.NodeID] = NodeStateIdle

	// 3. Save the event to db.
	e.ag.db.AddEvent(event.RunID, event)
}

func (gs *GraphState) SetNodeError(e *Execution, event *db.Event) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if gs.nodeStates[event.NodeID] != NodeStateRunning {
		panic("Error event: node not running or node state missing")
	}

	// 1. Add the error to the node errors (TODO: Should be list of errors)
	gs.nodeErrors[event.NodeID] = event.ErrorData.Error

	// 2. Update State to idle.
	gs.nodeStates[event.NodeID] = NodeStateIdle

	// 3. Save the event to db.
	e.ag.db.AddEvent(event.RunID, event)
}

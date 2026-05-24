package agentgraph

import (
	"fmt"
	"sync"
	"time"

	"github.com/thakee/orca/orca/runtime/db"
	"github.com/thakee/orca/orca/runtime/value"
)

const dispatchWebhookEnqueueTimeout = 100 * time.Millisecond

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

// GraphState is strictly not mutable by arbitrary code paths: it updates when the graph is
// loaded from the database and replayed, or when execution applies events (after a node is
// done and output is merged into state).
type GraphState struct {
	mu         sync.Mutex
	nodeStates map[string]NodeState
	nodeData   map[string]value.Value

	// TODO: Errors should be list of values (we accumilate all the errors during the execution)
	nodeErrors map[string]value.Value
}

// LoadGraphState rebuilds state by replaying persisted events for runID in order.
func LoadGraphState(runID string, db db.DB) (*GraphState, error) {
	graphState := &GraphState{
		nodeStates: make(map[string]NodeState),
		nodeData:   make(map[string]value.Value),
		nodeErrors: make(map[string]value.Value),
	}

	if db != nil {
		for i, event := range db.GetEvents(runID) {
			if err := graphState.replay(event); err != nil {
				return nil, fmt.Errorf("replay event %d (kind=%s): %w", i, event.Kind, err)
			}
		}
	}

	return graphState, nil
}

func (gs *GraphState) replay(event *db.Event) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	return transitionReplay(gs.nodeStates, gs.nodeData, gs.nodeErrors, event)
}

// RunNode marks the node running and persists the run_node event.
func (gs *GraphState) RunNode(e *Execution, event *db.Event) error {
	if event == nil {
		return fmt.Errorf("graphstate RunNode: nil event")
	}

	gs.mu.Lock()
	prev := stateOrIdle(gs.nodeStates, event.NodeID)
	if prev == NodeStateRunning && event.Persisted {
		gs.mu.Unlock()
		go e.RunNode(event.NodeID)
		return nil
	}
	if prev != NodeStateIdle && prev != NodeStateWebhookDispatched {
		gs.mu.Unlock()
		return fmt.Errorf("graphstate RunNode: node %q has invalid state %q (want idle or webhook_dispatched)", event.NodeID, prev)
	}

	// 1. Update State to running.
	gs.nodeStates[event.NodeID] = NodeStateRunning

	// 2. Save the event to db (unless caller already persisted it).
	if !event.Persisted {
		if err := e.ag.db.AddEvent(event.RunID, event); err != nil {
			gs.nodeStates[event.NodeID] = prev
			gs.mu.Unlock()
			return fmt.Errorf("graphstate RunNode: %w", err)
		}
	}

	// Mutex no longer needed below (RunNode may block); unlock before starting the goroutine.
	gs.mu.Unlock()

	// 3. Start the coroutine to run the node.
	go e.RunNode(event.NodeID)
	return nil
}

// SetListening moves a running node to listening (webhook / HIL).
func (gs *GraphState) SetListening(e *Execution, event *db.Event) error {
	if event == nil {
		return fmt.Errorf("graphstate SetListening: nil event")
	}

	gs.mu.Lock()
	prev := stateOrIdle(gs.nodeStates, event.NodeID)
	if prev != NodeStateRunning {
		gs.mu.Unlock()
		return fmt.Errorf("graphstate SetListening: node %q has invalid state %q (want running)", event.NodeID, prev)
	}

	// 1. Update State to listening.
	gs.nodeStates[event.NodeID] = NodeStateListening

	// 2. Save the event to db.
	if err := e.ag.db.AddEvent(event.RunID, event); err != nil {
		gs.nodeStates[event.NodeID] = prev
		gs.mu.Unlock()
		return fmt.Errorf("graphstate SetListening: %w", err)
	}
	gs.mu.Unlock()
	return nil
}

// DispatchWebhook applies an inbound webhook payload and optionally queues the node to run.
func (gs *GraphState) DispatchWebhook(e *Execution, event *db.Event) error {
	if e == nil || e.ag == nil || e.ag.db == nil || event == nil || event.WebhookData == nil {
		return fmt.Errorf("graphstate DispatchWebhook: invalid event")
	}

	gs.mu.Lock()
	st := stateOrIdle(gs.nodeStates, event.NodeID)
	webhookResponseMsg := "Webhook not listening"
	var runEv *db.Event
	switch st {
	case NodeStateListening:
		webhookResponseMsg = "Acknowledged"
		// 1. Save the event to db.
		if err := e.ag.db.AddEvent(event.RunID, event); err != nil {
			gs.mu.Unlock()
			respondWebhookEvent(event, "Failed to persist webhook")
			return fmt.Errorf("graphstate DispatchWebhook: %w", err)
		}
		// 2. Set the payload to the node data.
		gs.nodeData[event.NodeID] = event.WebhookData.Payload
		// 3. Update State to webhook_dispatched.
		gs.nodeStates[event.NodeID] = NodeStateWebhookDispatched
		runEv = db.NewRunNodeEvent(event.RunID, event.NodeID)
		if err := e.ag.db.AddEvent(event.RunID, runEv); err != nil {
			gs.mu.Unlock()
			respondWebhookEvent(event, "Failed to persist run scheduling")
			return fmt.Errorf("graphstate DispatchWebhook: %w", err)
		}
		runEv.Persisted = true
	case NodeStateWebhookDispatched:
		webhookResponseMsg = "Webhook already dispatched"
	}
	gs.mu.Unlock()

	if runEv != nil {
		if err := enqueueRunNodeEvent(e, runEv); err != nil {
			gs.mu.Lock()
			gs.nodeErrors[event.NodeID] = value.NewString(err.Error())
			gs.mu.Unlock()
			respondWebhookEvent(event, "Runtime busy, try again")
			return fmt.Errorf("graphstate DispatchWebhook: %w", err)
		}
	}
	respondWebhookEvent(event, webhookResponseMsg)

	return nil
}

func enqueueRunNodeEvent(e *Execution, runEv *db.Event) (err error) {
	if e == nil || runEv == nil {
		return fmt.Errorf("enqueue run_node: invalid input")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("enqueue run_node: inbox closed")
		}
	}()
	var ctxDone <-chan struct{}
	if e.ag != nil && e.ag.ctx != nil {
		ctxDone = e.ag.ctx.Done()
	}
	timer := time.NewTimer(dispatchWebhookEnqueueTimeout)
	defer timer.Stop()

	select {
	case e.eventInbox <- *runEv:
		return nil
	case <-ctxDone:
		return fmt.Errorf("enqueue run_node: execution shutting down")
	case <-timer.C:
		return fmt.Errorf("enqueue run_node: inbox full")
	}
}

func respondWebhookEvent(event *db.Event, msg string) {
	if event == nil || event.WebhookData == nil || event.WebhookData.RespondCallback == nil {
		return
	}
	event.WebhookData.RespondCallback(func(r chan any) {
		r <- msg
	})
}

// SetNodeDone merges the node result and returns to idle.
func (gs *GraphState) SetNodeDone(e *Execution, event *db.Event) error {
	if event == nil || event.DoneData == nil {
		return fmt.Errorf("graphstate SetNodeDone: invalid event")
	}

	gs.mu.Lock()
	prev := stateOrIdle(gs.nodeStates, event.NodeID)
	if prev != NodeStateRunning {
		gs.mu.Unlock()
		return fmt.Errorf("graphstate SetNodeDone: node %q has invalid state %q (want running)", event.NodeID, prev)
	}
	// 1. Save the event to db.
	if err := e.ag.db.AddEvent(event.RunID, event); err != nil {
		gs.mu.Unlock()
		return fmt.Errorf("graphstate SetNodeDone: %w", err)
	}
	// 2. Merge the result into the node data.
	gs.nodeData[event.NodeID] = value.Merge(gs.nodeData[event.NodeID], event.DoneData.Result)
	// 3. Update State to idle.
	gs.nodeStates[event.NodeID] = NodeStateIdle
	gs.mu.Unlock()
	return nil
}

// SetNodeError records a node error and returns to idle.
func (gs *GraphState) SetNodeError(e *Execution, event *db.Event) error {
	if event == nil || event.ErrorData == nil {
		return fmt.Errorf("graphstate SetNodeError: invalid event")
	}

	gs.mu.Lock()
	prev := stateOrIdle(gs.nodeStates, event.NodeID)
	if prev != NodeStateRunning {
		gs.mu.Unlock()
		return fmt.Errorf("graphstate SetNodeError: node %q has invalid state %q (want running)", event.NodeID, prev)
	}
	// 1. Save the event to db.
	if err := e.ag.db.AddEvent(event.RunID, event); err != nil {
		gs.mu.Unlock()
		return fmt.Errorf("graphstate SetNodeError: %w", err)
	}
	// 2. Add the error to the node errors (TODO: Should be list of errors)
	gs.nodeErrors[event.NodeID] = event.ErrorData.Error
	// 3. Update State to idle.
	gs.nodeStates[event.NodeID] = NodeStateIdle
	gs.mu.Unlock()
	return nil
}

// Snapshot returns shallow copies of maps for tests and debugging.
func (gs *GraphState) Snapshot() (states map[string]NodeState, data map[string]value.Value, errs map[string]value.Value) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	states = make(map[string]NodeState, len(gs.nodeStates))
	for k, v := range gs.nodeStates {
		states[k] = v
	}
	data = make(map[string]value.Value, len(gs.nodeData))
	for k, v := range gs.nodeData {
		data[k] = v
	}
	errs = make(map[string]value.Value, len(gs.nodeErrors))
	for k, v := range gs.nodeErrors {
		errs[k] = v
	}
	return states, data, errs
}

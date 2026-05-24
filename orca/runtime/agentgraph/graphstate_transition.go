package agentgraph

import (
	"fmt"

	"github.com/thakee/orca/orca/runtime/db"
	"github.com/thakee/orca/orca/runtime/value"
)

// stateOrIdle returns the persisted node state, or idle when the node has never been recorded.
func stateOrIdle(nodeStates map[string]NodeState, nodeID string) NodeState {
	if s, ok := nodeStates[nodeID]; ok {
		return s
	}
	return NodeStateIdle
}

// transitionReplay applies one persisted event when rebuilding GraphState from the event log.
// It mutates nodeStates, nodeData, and nodeErrors. Unknown event kinds are ignored (no-op).
func transitionReplay(
	nodeStates map[string]NodeState,
	nodeData map[string]value.Value,
	nodeErrors map[string]value.Value,
	ev *db.Event,
) error {
	if ev == nil {
		return fmt.Errorf("graphstate replay: nil event")
	}

	switch ev.Kind {
	case db.EventTypeWebhook:
		if ev.WebhookData == nil {
			return fmt.Errorf("graphstate replay: webhook event %q missing WebhookData", ev.NodeID)
		}
		if stateOrIdle(nodeStates, ev.NodeID) != NodeStateListening {
			return fmt.Errorf("graphstate replay: webhook for node %q requires listening, got %q",
				ev.NodeID, stateOrIdle(nodeStates, ev.NodeID))
		}
		nodeData[ev.NodeID] = ev.WebhookData.Payload
		nodeStates[ev.NodeID] = NodeStateWebhookDispatched

	case db.EventTypeMsgStream:
		// UI-only; does not affect execution state.
		return nil

	case db.EventTypeListening:
		if stateOrIdle(nodeStates, ev.NodeID) != NodeStateRunning {
			return fmt.Errorf("graphstate replay: listening for node %q requires running, got %q",
				ev.NodeID, stateOrIdle(nodeStates, ev.NodeID))
		}
		nodeStates[ev.NodeID] = NodeStateListening

	case db.EventTypeDone:
		if ev.DoneData == nil {
			return fmt.Errorf("graphstate replay: done event %q missing DoneData", ev.NodeID)
		}
		if stateOrIdle(nodeStates, ev.NodeID) != NodeStateRunning {
			return fmt.Errorf("graphstate replay: done for node %q requires running, got %q",
				ev.NodeID, stateOrIdle(nodeStates, ev.NodeID))
		}
		nodeData[ev.NodeID] = value.Merge(nodeData[ev.NodeID], ev.DoneData.Result)
		nodeStates[ev.NodeID] = NodeStateIdle

	case db.EventTypeError:
		if ev.ErrorData == nil {
			return fmt.Errorf("graphstate replay: error event %q missing ErrorData", ev.NodeID)
		}
		if stateOrIdle(nodeStates, ev.NodeID) != NodeStateRunning {
			return fmt.Errorf("graphstate replay: error for node %q requires running, got %q",
				ev.NodeID, stateOrIdle(nodeStates, ev.NodeID))
		}
		nodeErrors[ev.NodeID] = ev.ErrorData.Error
		nodeStates[ev.NodeID] = NodeStateIdle

	case db.EventTypeRunNode:
		prev := stateOrIdle(nodeStates, ev.NodeID)
		if prev != NodeStateIdle && prev != NodeStateWebhookDispatched {
			return fmt.Errorf("graphstate replay: run_node for node %q invalid prior state %q",
				ev.NodeID, prev)
		}
		nodeStates[ev.NodeID] = NodeStateRunning

	default:
		// Unknown kinds: tolerate forward compatibility.
		return nil
	}

	return nil
}

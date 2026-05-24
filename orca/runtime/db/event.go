package db

import (
	"time"

	"github.com/thakee/orca/orca/runtime/value"
)

type EventKind string

const (
	EventTypeRunNode   EventKind = "run_node"
	EventTypeListening EventKind = "listening"
	EventTypeWebhook   EventKind = "webhook"
	EventTypeMsgStream EventKind = "stream"
	EventTypeDone      EventKind = "done"
	EventTypeError     EventKind = "error"
)

type Event struct {
	Kind EventKind `json:"kind"`
	Time time.Time `json:"time"`

	// This will be empty string if the event is not related to a specific run or node.
	RunID  string `json:"run_id"`
	NodeID string `json:"node_id"`

	WebhookData *EventWebhookData   `json:"webhook_data"`
	StreamData  *EventMsgStreamData `json:"stream_data"`
	DoneData    *EventDoneData      `json:"done_data"`
	ErrorData   *EventErrorData     `json:"error_data"`

	// Persisted indicates the event was already written to DB before enqueueing.
	// This is runtime-only metadata and is intentionally not serialized.
	Persisted bool `json:"-"`
}

type EventWebhookData struct {
	Payload value.Value `json:"payload"`

	// The function to respond to the webhook request.
	// Example use:
	// webhookEvent.RespondFunc(func (stream chan any) {
	//   stream <- "Acknowledged"
	// })
	RespondCallback func(func(chan any)) `json:"-"`
}

type EventMsgStreamData struct {
	StreamKind string `json:"stream_kind"`
	Content    string `json:"content"`
}

type EventDoneData struct {
	RunID  string      `json:"run_id"`
	NodeID string      `json:"node_id"`
	Result value.Value `json:"result"`
}

type EventErrorData struct {
	RunID  string      `json:"run_id"`
	NodeID string      `json:"node_id"`
	Error  value.Value `json:"error"`
}

func NewRunNodeEvent(runID string, nodeID string) *Event {
	return &Event{
		Kind:   EventTypeRunNode,
		Time:   time.Now(),
		RunID:  runID,
		NodeID: nodeID,
	}
}

func NewNodeEventWebhook(runID string, nodeID string, data value.Value) *Event {
	return &Event{
		Kind:   EventTypeWebhook,
		Time:   time.Now(),
		RunID:  runID,
		NodeID: nodeID,
		WebhookData: &EventWebhookData{
			Payload: data,
		},
	}
}

func NewNodeEventMsgStream(runID string, nodeID string, streamKind string, content string) *Event {
	return &Event{
		Kind:   EventTypeMsgStream,
		Time:   time.Now(),
		RunID:  runID,
		NodeID: nodeID,
		StreamData: &EventMsgStreamData{
			StreamKind: streamKind,
			Content:    content,
		},
	}
}

func NewNodeEventDone(runID string, nodeID string, result value.Value) *Event {
	return &Event{
		Kind:   EventTypeDone,
		Time:   time.Now(),
		RunID:  runID,
		NodeID: nodeID,
		DoneData: &EventDoneData{
			Result: result,
		},
	}
}

func NewNodeEventError(runID string, nodeID string, err value.Value) *Event {
	return &Event{
		Kind:   EventTypeError,
		Time:   time.Now(),
		RunID:  runID,
		NodeID: nodeID,
		ErrorData: &EventErrorData{
			Error: err,
		},
	}
}

package db

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thakee/orca/orca/runtime/value"
)

func TestNewRunNodeEvent(t *testing.T) {
	e := NewRunNodeEvent("run1", "nodeA")
	if e.Kind != EventTypeRunNode || e.RunID != "run1" || e.NodeID != "nodeA" {
		t.Fatalf("got %+v", e)
	}
	if e.Time.IsZero() {
		t.Fatal("Time should be set")
	}
}

func TestNewNodeEventWebhook(t *testing.T) {
	payload := value.NewString("body")
	e := NewNodeEventWebhook("r", "hook", payload)
	got, ok := e.WebhookData.Payload.String()
	want, _ := payload.String()
	if !ok || got != want {
		t.Fatalf("payload String() = %q,%v, want %q", got, ok, want)
	}
}

func TestNewNodeEventMsgStream(t *testing.T) {
	e := NewNodeEventMsgStream("r", "n", "token", "hello")
	if e.Kind != EventTypeMsgStream {
		t.Fatal(e.Kind)
	}
	if e.StreamData.StreamKind != "token" || e.StreamData.Content != "hello" {
		t.Fatal(e.StreamData)
	}
}

func TestNewNodeEventDone(t *testing.T) {
	res := value.NewNumber(42)
	e := NewNodeEventDone("r", "n", res)
	got, ok := e.DoneData.Result.Number()
	if !ok || got != 42 {
		t.Fatalf("Done result number = (%v,%v)", got, ok)
	}
}

func TestNewNodeEventError(t *testing.T) {
	errVal := value.NewString("oops")
	e := NewNodeEventError("r", "n", errVal)
	got, ok := e.ErrorData.Error.String()
	if !ok || got != "oops" {
		t.Fatalf("error string = (%q,%v)", got, ok)
	}
}

func TestEventJSONOmitsRespondCallback(t *testing.T) {
	e := NewNodeEventWebhook("r", "n", value.NewNil())
	e.WebhookData.RespondCallback = func(func(chan any)) {}

	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "RespondCallback") {
		t.Fatalf("JSON must omit RespondCallback: %s", b)
	}
}

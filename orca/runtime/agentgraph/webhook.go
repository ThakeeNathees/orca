package agentgraph

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/thakee/orca/orca/compiler/helper"
	"github.com/thakee/orca/orca/runtime/db"
	"github.com/thakee/orca/orca/runtime/value"
)

// Run ID for inbound HTTP webhooks (query parameter).
const webhookRunIDQueryParam = "run_id"

const maxWebhookBodyBytes = 1 << 20
const webhookIngressEnqueueTimeout = 100 * time.Millisecond

var (
	errWebhookIngressShuttingDown = errors.New("runtime shutting down")
	errWebhookIngressOverloaded   = errors.New("runtime overloaded")
)

func normalizeWebhookPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return path.Clean(p)
}

func stringFieldFromBlock(v value.Value, field string) (string, bool) {
	keys, vals, ok := v.KeysValues()
	if !ok {
		return "", false
	}
	for i := range keys {
		k, ko := keys[i].String()
		if ko && k == field {
			s, vo := vals[i].String()
			return s, vo
		}
	}
	return "", false
}

// GetWebhookHandler builds HTTP registration for a webhook trigger node. nodeID is the
// workflow trigger name; v must be a materialized webhook block (KindBlock, kind webhook).
func GetWebhookHandler(ag *AgentGraph, nodeID string, v *value.Value) helper.WebhookHandler {
	pathStr, _ := stringFieldFromBlock(*v, "path")
	ep := normalizeWebhookPath(pathStr)

	return helper.WebhookHandler{
		Endpoint: ep,
		Handler: func(w http.ResponseWriter, r *http.Request, stream chan any) {
			_ = w // response streamed via RespondCallback / JSON encoder in cmd/start
			runID := r.URL.Query().Get(webhookRunIDQueryParam)
			if runID == "" {
				stream <- map[string]string{"error": "missing run_id query parameter"}
				return
			}

			if want, ok := stringFieldFromBlock(*v, "method"); ok && strings.TrimSpace(want) != "" {
				if !strings.EqualFold(r.Method, strings.TrimSpace(want)) {
					stream <- map[string]string{"error": "HTTP method does not match webhook.method"}
					return
				}
			}

			limited := io.LimitReader(r.Body, maxWebhookBodyBytes+1)
			body, err := io.ReadAll(limited)
			if err != nil {
				stream <- map[string]string{"error": err.Error()}
				return
			}
			if len(body) > maxWebhookBodyBytes {
				stream <- map[string]string{"error": "request body too large"}
				return
			}

			payload, err := value.ValueFromJSON(body)
			if err != nil {
				stream <- map[string]string{"error": err.Error()}
				return
			}

			ev := db.NewNodeEventWebhook(runID, nodeID, payload)
			ev.WebhookData.RespondCallback = func(inner func(chan any)) {
				inner(stream)
			}

			if err := enqueueWebhookEvent(ag, ev); err != nil {
				stream <- map[string]string{"error": err.Error()}
			}
		},
	}
}

func enqueueWebhookEvent(ag *AgentGraph, ev *db.Event) error {
	if ag == nil || ag.events == nil || ev == nil {
		return fmt.Errorf("runtime unavailable")
	}
	var ctxDone <-chan struct{}
	if ag.ctx != nil {
		ctxDone = ag.ctx.Done()
		select {
		case <-ctxDone:
			return errWebhookIngressShuttingDown
		default:
		}
	}
	timer := time.NewTimer(webhookIngressEnqueueTimeout)
	defer timer.Stop()

	select {
	case ag.events <- *ev:
		return nil
	case <-ctxDone:
		return errWebhookIngressShuttingDown
	case <-timer.C:
		return errWebhookIngressOverloaded
	}
}

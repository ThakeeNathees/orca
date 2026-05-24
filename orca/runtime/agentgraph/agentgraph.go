// Package agentgraph provides the orca runtime, a graph based runtime, Initially used
// langgraph as the underlying graph framework however it doesnt have enough control for
// orca and we have implemented our own graph framework.
package agentgraph

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thakee/orca/orca/compiler/analyzer"
	"github.com/thakee/orca/orca/compiler/ast"
	"github.com/thakee/orca/orca/compiler/graph"
	"github.com/thakee/orca/orca/compiler/helper"
	"github.com/thakee/orca/orca/compiler/types"
	"github.com/thakee/orca/orca/compiler/workflow"
	"github.com/thakee/orca/orca/runtime/db"
	"github.com/thakee/orca/orca/runtime/value"
)

type WebhookListenerFunc func(port int, webhooks []helper.WebhookHandler) error

const routeEventToExecutionTimeout = 100 * time.Millisecond

var (
	errRouteExecutionShuttingDown = errors.New("route event: execution shutting down")
	errRouteExecutionBackpressure = errors.New("route event: execution inbox full")
)

// The configuration to create a new agent graph
type AgentGraphConfig struct {
	Prog                 *analyzer.AnalyzedProgram
	WorkflowBlock        *ast.BlockStatement
	Db                   db.DB
	EventBufferSize      int
	Logger               *log.Logger
	StartWebhookListener WebhookListenerFunc
}

// AgentGraph is the main entry point for the agent graph. It contains the program, graph, db, and context.
type AgentGraph struct {
	// The context and event loop.
	ctx                  context.Context
	ctxStop              context.CancelFunc
	eventsBuffSize       int
	events               chan db.Event
	logger               *log.Logger
	webhookPort          int
	startWebhookListener func(port int, webhooks []helper.WebhookHandler) error
	streamHandler        func(event *db.Event)

	// The graph blueprint.
	graph    *graph.Graph[string]
	stack    *value.Stack
	db       db.DB
	prog     *analyzer.AnalyzedProgram
	resolved workflow.ResolvedWorkflow
}

func NewAgentGraph(config AgentGraphConfig) *AgentGraph {

	// FIXME: We need to persist the analyzed program in databse for long running execution
	// cause the file might be modified / deleted and a new version of the workflow will be used.
	// So each agent graph should have its own analyzed program (versioned).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	rw := workflow.ResolveV2(config.Prog, config.WorkflowBlock)
	g := buildAgentGraph(&rw)
	vm := buildRuntimeValueMap(config.Prog, &rw)
	stk := value.NewStack(nil, vm)

	return &AgentGraph{
		ctx:                  ctx,
		ctxStop:              stop,
		eventsBuffSize:       config.EventBufferSize,
		events:               make(chan db.Event, max(1, config.EventBufferSize)),
		logger:               config.Logger,
		webhookPort:          getWebhookPort(&config),
		graph:                g,
		stack:                stk,
		db:                   config.Db,
		prog:                 config.Prog,
		resolved:             rw,
		startWebhookListener: config.StartWebhookListener,
	}
}

func (ag *AgentGraph) StartMainLoop() error {

	// Listen for Ctrl+C or SIGTERM and stop the execution.
	defer ag.ctxStop()

	executionStates := loadExecutions(ag)
	startExecutionLoops(executionStates)
	defer stopExecutionLoops(executionStates)

	// Start the external event listenerr loop, it can be configured with
	// and HTTP server or just callbacks etc.
	go ag.listenForEvents()

	for {
		select {
		case <-ag.ctx.Done():
			return nil // TODO: Not sure what to return here.

		case event := <-ag.events:
			ag.logEvent(&event)
			ag.handleEvent(&event, executionStates)
		}

	}
}

// startExecutionLoops starts one event-processing goroutine per loaded execution.
func startExecutionLoops(executions map[string]*Execution) {
	for _, execution := range executions {
		go execution.Execute()
		go execution.recoverPendingNodes()
	}
}

// stopExecutionLoops closes execution inboxes so Execute loops can exit cleanly.
func stopExecutionLoops(executions map[string]*Execution) {
	for _, execution := range executions {
		close(execution.eventInbox)
	}
}

func buildAgentGraph(rw *workflow.ResolvedWorkflow) *graph.Graph[string] {
	graph := graph.New[string]()
	for _, node := range rw.Nodes {
		graph.AddNode(node)
	}
	for _, edge := range rw.Edges {
		graph.AddEdge(edge.From, edge.To)
	}
	return graph
}

func getWebhookPort(config *AgentGraphConfig) int {
	if config.WorkflowBlock == nil {
		return types.DefaultWebhookPort
	}
	if expr, ok := config.WorkflowBlock.GetFieldExpression(types.WebhookPortField); ok {
		cv, diags := analyzer.ConstFold(expr, config.Prog)
		if len(diags) == 0 && cv.Kind == analyzer.ConstNumber {
			return int(cv.Number)
		}
	}
	return types.DefaultWebhookPort
}

func (ag *AgentGraph) getWebhookHandlers() []helper.WebhookHandler {
	var out []helper.WebhookHandler
	if ag.prog == nil {
		return out
	}
	for _, trig := range ag.resolved.Triggers {
		typ, ok := ag.prog.SymbolTable.Lookup(trig)
		if !ok || !types.IsBlockKind(typ, types.BlockKindWebhook) {
			continue
		}
		nodeVal := ag.LookupValue(trig)
		if nodeVal == nil {
			if ag.logger != nil {
				ag.logger.Printf("webhook trigger %q: no runtime value", trig)
			}
			continue
		}
		if !nodeVal.IsWebhookNode() {
			continue
		}
		out = append(out, GetWebhookHandler(ag, trig, nodeVal))
	}
	return out
}

func (ag *AgentGraph) listenForEvents() {
	if ag.startWebhookListener == nil {
		return
	}
	ag.startWebhookListener(ag.webhookPort, ag.getWebhookHandlers())
}

func (ag *AgentGraph) logEvent(event *db.Event) {
	if ag.logger == nil {
		return
	}
	// TODO: log the event better.
	ag.logger.Printf("received event: %+v", event)
}

func (ag *AgentGraph) handleEvent(event *db.Event, executionStates map[string]*Execution) {
	execution, err := ag.getOrCreateExecution(event, executionStates)
	if err != nil {
		if event != nil && event.Kind == db.EventTypeWebhook {
			respondWebhookMissingExecution(event, "execution not found for run_id")
		}
		if ag.logger != nil {
			ag.logger.Printf("failed to get execution: %v, event: %+v", err, event)
		}
		return
	}
	if err := ag.routeEventToExecution(event, execution); err != nil {
		if event != nil && event.Kind == db.EventTypeWebhook {
			msg := "runtime busy, try again"
			if errors.Is(err, errRouteExecutionShuttingDown) {
				msg = "runtime shutting down"
			}
			respondWebhookMissingExecution(event, msg)
		}
		if ag.logger != nil {
			ag.logger.Printf("failed to route event: %v, event: %+v", err, event)
		}
	}
}

// getOrCreateExecution returns an existing execution for event.RunID, or lazily creates one.
func (ag *AgentGraph) getOrCreateExecution(event *db.Event, executionStates map[string]*Execution) (*Execution, error) {
	if event == nil {
		return nil, fmt.Errorf("nil event")
	}
	if event.RunID == "" {
		return nil, fmt.Errorf("empty run_id")
	}
	if execution, ok := executionStates[event.RunID]; ok {
		return execution, nil
	}
	if ag.db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	if err := ag.db.EnsureExecution(event.RunID); err != nil {
		return nil, fmt.Errorf("ensure execution for run ID %q: %w", event.RunID, err)
	}
	execution, err := LoadExecution(ag, event.RunID)
	if err != nil {
		return nil, fmt.Errorf("load execution for run ID %q: %w", event.RunID, err)
	}
	executionStates[event.RunID] = execution
	go execution.Execute()
	return execution, nil
}

// routeEventToExecution hands ownership of event processing to the execution loop.
// Returns an error when routing cannot complete due to shutdown or backpressure.
func (ag *AgentGraph) routeEventToExecution(event *db.Event, execution *Execution) error {
	if event == nil || execution == nil {
		return fmt.Errorf("route event: invalid input")
	}
	var ctxDone <-chan struct{}
	if ag.ctx != nil {
		ctxDone = ag.ctx.Done()
		select {
		case <-ctxDone:
			return errRouteExecutionShuttingDown
		default:
		}
	}
	timer := time.NewTimer(routeEventToExecutionTimeout)
	defer timer.Stop()
	select {
	case execution.eventInbox <- *event:
		return nil
	case <-ctxDone:
		return errRouteExecutionShuttingDown
	case <-timer.C:
		return errRouteExecutionBackpressure
	}
}

func (ag *AgentGraph) handleStreamEvent(event *db.Event) {
	if ag.streamHandler != nil {
		ag.streamHandler(event)
	}
}

func (ag *AgentGraph) LookupValue(name string) *value.Value {
	if ag.stack == nil {
		return nil
	}
	v, ok := ag.stack.Lookup(name)
	if !ok {
		return nil
	}
	return v
}

// respondWebhookMissingExecution invokes the HTTP respond callback so the client does not hang.
func respondWebhookMissingExecution(ev *db.Event, msg string) {
	if ev == nil || ev.WebhookData == nil || ev.WebhookData.RespondCallback == nil {
		return
	}
	ev.WebhookData.RespondCallback(func(ch chan any) {
		ch <- msg
	})
}

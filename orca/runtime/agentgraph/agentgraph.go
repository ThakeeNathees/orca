// Package agentgraph provides the orca runtime, a graph based runtime, Initially used
// langgraph as the underlying graph framework however it doesnt have enough control for
// orca and we have implemented our own graph framework.
package agentgraph

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

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
	graph *graph.Graph[string]
	stack *value.Stack
	db    db.DB
}

func NewAgentGraph(config AgentGraphConfig) *AgentGraph {

	// FIXME: We need to persist the analyzed program in databse for long running execution
	// cause the file might be modified / deleted and a new version of the workflow will be used.
	// So each agent graph should have its own analyzed program (versioned).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	rw := workflow.ResolveV2(config.Prog, config.WorkflowBlock)
	graph := buildAgentGraph(&rw)

	return &AgentGraph{
		ctx:            ctx,
		ctxStop:        stop,
		eventsBuffSize: config.EventBufferSize,
		events:         make(chan db.Event, max(1, config.EventBufferSize)),
		logger:         config.Logger,
		webhookPort:    getWebhookPort(&config),
		graph:          graph,
		stack:          stack,
		db:             config.Db,
	}
}

func (ag *AgentGraph) StartMainLoop() error {

	// Listen for Ctrl+C or SIGTERM and stop the execution.
	defer ag.ctxStop()

	executionStates := loadExecutions(ag)

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
	if expr, ok := config.WorkflowBlock.GetFieldExpression(types.WebhookPortField); ok {
		val, _ := analyzer.ConstFold(expr, config.Prog)
		if val.Kind == analyzer.ConstNumber {
			return int(val.Number)
		}
	}
	return types.DefaultWebhookPort
}

func (ag *AgentGraph) getWebhookHandlers() []helper.WebhookHandler {
	webhookHandlers := []helper.WebhookHandler{}
	for _, nodeName := range ag.graph.Nodes() {
		if nodeValue := ag.LookupValue(nodeName); nodeValue != nil {
			if nodeValue.IsWebhookNode() {
				webhookHandlers = append(webhookHandlers, GetWebhookHandlerOf(nodeValue))
			}
		} else {
			ag.logger.Printf("node object not found for node name: %s", nodeName)
		}
	}
	return webhookHandlers
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
	execution, ok := executionStates[event.RunID]
	if !ok {
		ag.logger.Printf("execution not found for run ID: %s, event: %+v", event.RunID, event)
		return
	}
	if err := execution.handleEvent(event); err != nil {
		ag.logger.Printf("error handling event: %s, event: %+v", err, event)
	}
}

func (ag *AgentGraph) handleStreamEvent(event *db.Event) {
	if ag.streamHandler != nil {
		ag.streamHandler(event)
	}
}

func (ag *AgentGraph) LookupValue(name string) *value.Value {
}

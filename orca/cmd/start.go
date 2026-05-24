package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/thakee/orca/orca/compiler/analyzer"
	"github.com/thakee/orca/orca/compiler/ast"
	"github.com/thakee/orca/orca/compiler/diagnostic"
	"github.com/thakee/orca/orca/compiler/helper"
	"github.com/thakee/orca/orca/compiler/lexer"
	"github.com/thakee/orca/orca/compiler/parser"
	"github.com/thakee/orca/orca/compiler/types"
	"github.com/thakee/orca/orca/runtime/agentgraph"
	"github.com/thakee/orca/orca/runtime/db"
)

// startCmd builds and then runs the compiled output.
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Orca server",
	Long:  "Compiles .orca files and starts the Orca server.",
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	files, err := filepath.Glob("*.orca")
	if err != nil {
		return fmt.Errorf("failed to find .orca files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no .orca files found in current directory")
	}

	// TODO: abstract this to a function probably in the compiler
	// (make sure its pure i.e. Dont read files but only process the given source)
	//
	// Parse each file separately to preserve per-file line numbers.
	// sources keeps each file's text keyed by path so analyzer/codegen
	// diagnostics (which reference blocks by SourceFile) can still be
	// rendered with source context after the parse loop finishes.
	sources := make(map[string]string, len(files))
	var program ast.Program
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}
		src := string(data)
		sources[file] = src

		l := lexer.New(src, file)
		p := parser.New(l)
		fileProg := p.ParseProgram()

		if len(p.Diagnostics()) > 0 {
			for _, d := range p.Diagnostics() {
				fmt.Fprintln(os.Stderr, diagnostic.Render(src, d))
			}
			return fmt.Errorf("compilation failed with parse errors")
		}

		program.Statements = append(program.Statements, fileProg.Statements...)
	}

	// Run semantic analysis.
	analyzedProg := analyzer.Analyze(&program)
	if len(analyzedProg.Diagnostics) > 0 {
		if reportDiagnostics(analyzedProg.Diagnostics, sources) {
			return fmt.Errorf("compilation failed with analysis errors")
		}
	}

	wfBlock, err := findWorkflowBlock(analyzedProg.Ast)
	if err != nil {
		return err
	}

	// Create the agent graph.
	agentGraph := agentgraph.NewAgentGraph(agentgraph.AgentGraphConfig{
		Prog:                 &analyzedProg,
		WorkflowBlock:        wfBlock,
		Db:                   db.NewInMemoryDB(),
		EventBufferSize:      100,
		Logger:               log.New(os.Stdout, "agentgraph: ", log.LstdFlags|log.Lmsgprefix),
		StartWebhookListener: startWebhookListener,
	})

	// Start the agent graph.
	return agentGraph.StartMainLoop()
}

// findWorkflowBlock returns the first top-level workflow block in the program.
func findWorkflowBlock(prog *ast.Program) (*ast.BlockStatement, error) {
	if prog == nil {
		return nil, fmt.Errorf("program is nil")
	}
	for _, stmt := range prog.Statements {
		b, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		if b.Kind == types.BlockKindWorkflow {
			return b, nil
		}
	}
	return nil, fmt.Errorf("no workflow block found")
}

func startWebhookListener(port int, webhooks []helper.WebhookHandler) error {

	// Register endpoints for each webhook.
	for i := range webhooks {

		endpoint := webhooks[i].Endpoint
		handler := webhooks[i].Handler

		http.HandleFunc(endpoint, func(w http.ResponseWriter, r *http.Request) {
			streaming := helper.HttpStreamingHandler(func(w http.ResponseWriter, f http.Flusher) {
				respChan := make(chan any)
				go func() {
					defer close(respChan)
					handler(w, r, respChan)
				}()

				encoder := json.NewEncoder(w)
				for data := range respChan {
					if err := encoder.Encode(data); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					f.Flush()
				}
			})
			streaming(w, r)
		})
	}

	// Start the HTTP server (blocking).
	portStr := ":" + strconv.Itoa(port)
	return http.ListenAndServe(portStr, nil)
}

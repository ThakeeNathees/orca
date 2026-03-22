// Package lsp implements the Language Server Protocol server for Orca.
// It provides real-time diagnostics (parse errors) as users edit .oc files.
package lsp

import (
	"fmt"

	"github.com/tliron/commonlog"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	glspserver "github.com/tliron/glsp/server"

	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
)

const serverName = "orca-lsp"

// handler is the LSP protocol handler with all method implementations.
var handler protocol.Handler

// documents tracks open file contents by URI, needed because
// didChange sends incremental updates and we need full text for parsing.
var documents = make(map[string]string)

func init() {
	handler.Initialize = initialize
	handler.Initialized = initialized
	handler.Shutdown = shutdown
	handler.TextDocumentDidOpen = textDocumentDidOpen
	handler.TextDocumentDidChange = textDocumentDidChange
	handler.TextDocumentDidClose = textDocumentDidClose
}

// NewServer creates a glsp server wired to the Orca LSP handler.
func NewServer() *glspserver.Server {
	return glspserver.NewServer(&handler, serverName, false)
}

// initialize responds to the client's initialize request with server capabilities.
func initialize(ctx *glsp.Context, params *protocol.InitializeParams) (any, error) {
	commonlog.NewInfoMessage(0, "orca-lsp initializing")

	syncKind := protocol.TextDocumentSyncKindFull

	return protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: boolPtr(true),
				Change:    &syncKind,
			},
		},
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    serverName,
			Version: strPtr("0.1.0"),
		},
	}, nil
}

// initialized is called after the client confirms initialization.
func initialized(ctx *glsp.Context, params *protocol.InitializedParams) error {
	return nil
}

// shutdown is called when the client asks the server to shut down.
func shutdown(ctx *glsp.Context) error {
	return nil
}

// textDocumentDidOpen is called when the client opens a file.
// We store the content and run initial diagnostics.
func textDocumentDidOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	uri := params.TextDocument.URI
	documents[uri] = params.TextDocument.Text
	publishDiagnostics(ctx, uri, params.TextDocument.Text)
	return nil
}

// textDocumentDidChange is called when the client modifies a file.
// We update stored content and re-run diagnostics.
func textDocumentDidChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	uri := params.TextDocument.URI
	// With full sync, the entire content is in the first change event.
	if len(params.ContentChanges) > 0 {
		change := params.ContentChanges[0]
		if changeEvent, ok := change.(protocol.TextDocumentContentChangeEventWhole); ok {
			documents[uri] = changeEvent.Text
			publishDiagnostics(ctx, uri, changeEvent.Text)
		}
	}
	return nil
}

// textDocumentDidClose is called when the client closes a file.
// We clear diagnostics and remove stored content.
func textDocumentDidClose(ctx *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	uri := params.TextDocument.URI
	delete(documents, uri)
	// Clear diagnostics for closed file.
	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: []protocol.Diagnostic{},
	})
	return nil
}

// publishDiagnostics parses the given source text and sends any parse
// errors to the client as LSP diagnostics.
func publishDiagnostics(ctx *glsp.Context, uri string, text string) {
	diagnostics := Diagnose(text)
	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}

// Diagnose parses source text and returns LSP diagnostics for any errors.
// Exported so it can be tested independently of the LSP transport.
func Diagnose(text string) []protocol.Diagnostic {
	l := lexer.New(text)
	p := parser.New(l)
	p.ParseProgram()

	// Must be an empty slice (not nil) so it marshals to [] in JSON.
	// LSP clients treat null as "no change" but [] as "clear all".
	diagnostics := []protocol.Diagnostic{}
	severity := protocol.DiagnosticSeverityError
	source := "orca"

	for _, errMsg := range p.Errors() {
		diagnostics = append(diagnostics, protocol.Diagnostic{
			Range:    errMsgToRange(errMsg),
			Severity: &severity,
			Source:   &source,
			Message:  errMsg,
		})
	}

	return diagnostics
}

// errMsgToRange extracts line/col from parser error messages (format: "line N, col M: ...")
// and converts to an LSP Range. Falls back to 0:0 if parsing fails.
func errMsgToRange(msg string) protocol.Range {
	var line, col int
	// Parser errors are formatted as "line %d, col %d: %s"
	_, _ = parseLineCol(msg, &line, &col)
	// LSP lines and columns are 0-based, parser uses 1-based.
	if line > 0 {
		line--
	}
	if col > 0 {
		col--
	}
	pos := protocol.Position{Line: protocol.UInteger(line), Character: protocol.UInteger(col)}
	return protocol.Range{Start: pos, End: pos}
}

// parseLineCol extracts line and column numbers from a parser error string.
func parseLineCol(msg string, line, col *int) (int, error) {
	var rest string
	n, err := fmt.Sscanf(msg, "line %d, col %d: %s", line, col, &rest)
	return n, err
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool { return &b }

// strPtr returns a pointer to a string value.
func strPtr(s string) *string { return &s }

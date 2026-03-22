// Package lsp implements the Language Server Protocol server for Orca.
// It provides real-time diagnostics, autocompletion, and other editor
// features for .oc files.
package lsp

import (
	"github.com/tliron/commonlog"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	glspserver "github.com/tliron/glsp/server"

	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/cursor"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
	"github.com/thakee/orca/compiler/types"
)

const serverName = "orca-lsp"

// handler is the LSP protocol handler with all method implementations.
var handler protocol.Handler

// documentState holds the current text and last successful AST for an open file.
type documentState struct {
	Text    string
	Program *ast.Program // nil if last parse had errors
}

// documents tracks open file state by URI.
var documents = make(map[string]*documentState)

func init() {
	handler.Initialize = initialize
	handler.Initialized = initialized
	handler.Shutdown = shutdown
	handler.TextDocumentDidOpen = textDocumentDidOpen
	handler.TextDocumentDidChange = textDocumentDidChange
	handler.TextDocumentDidClose = textDocumentDidClose
	handler.TextDocumentCompletion = textDocumentCompletion
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
			CompletionProvider: &protocol.CompletionOptions{},
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
// We store the content, parse, and run initial diagnostics.
func textDocumentDidOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	uri := params.TextDocument.URI
	doc := updateDocument(uri, params.TextDocument.Text)
	publishDiagnostics(ctx, uri, doc)
	return nil
}

// textDocumentDidChange is called when the client modifies a file.
// We update stored content, re-parse, and re-run diagnostics.
func textDocumentDidChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	uri := params.TextDocument.URI
	// With full sync, the entire content is in the first change event.
	if len(params.ContentChanges) > 0 {
		change := params.ContentChanges[0]
		if changeEvent, ok := change.(protocol.TextDocumentContentChangeEventWhole); ok {
			doc := updateDocument(uri, changeEvent.Text)
			publishDiagnostics(ctx, uri, doc)
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

// textDocumentCompletion handles completion requests. Uses cursor.Resolve
// to determine the context and returns appropriate completion items.
func textDocumentCompletion(ctx *glsp.Context, params *protocol.CompletionParams) (any, error) {
	doc, ok := documents[params.TextDocument.URI]
	if !ok || doc.Program == nil {
		return nil, nil
	}

	// LSP positions are 0-based, compiler positions are 1-based.
	line := int(params.Position.Line) + 1
	col := int(params.Position.Character) + 1

	cursorCtx := cursor.Resolve(doc.Program, line, col)
	return completionItems(cursorCtx), nil
}

// completionItems builds LSP completion items from a cursor context.
func completionItems(ctx cursor.Context) []protocol.CompletionItem {
	switch ctx.Position {
	case cursor.BlockBody:
		return completeFieldNames(ctx)
	case cursor.FieldValue:
		// TODO: suggest values based on field type (block refs, booleans, etc.)
		return nil
	default:
		return nil
	}
}

// completeFieldNames returns completion items for field names within a block,
// filtering out fields already assigned.
func completeFieldNames(ctx cursor.Context) []protocol.CompletionItem {
	if ctx.Schema == nil {
		return nil
	}

	// Collect already-assigned field names.
	assigned := make(map[string]bool)
	if ctx.Block != nil {
		for _, a := range ctx.Block.Assignments {
			assigned[a.Name] = true
		}
	}

	var items []protocol.CompletionItem
	for name, field := range ctx.Schema.Fields {
		if assigned[name] {
			continue
		}

		kind := protocol.CompletionItemKindField
		detail := typeString(field.Type)
		if field.Required {
			detail += " (required)"
		}

		item := protocol.CompletionItem{
			Label:  name,
			Kind:   &kind,
			Detail: &detail,
		}

		if field.Description != "" {
			doc := protocol.MarkupContent{
				Kind:  protocol.MarkupKindPlainText,
				Value: field.Description,
			}
			item.Documentation = doc
		}

		// Insert "field = " with cursor after the equals sign.
		insertText := name + " = "
		item.InsertText = &insertText

		// Sort required fields before optional ones.
		if field.Required {
			sortText := "0_" + name
			item.SortText = &sortText
		} else {
			sortText := "1_" + name
			item.SortText = &sortText
		}

		items = append(items, item)
	}

	return items
}

// typeString returns a human-readable string for a type, used in completion detail.
func typeString(t types.Type) string {
	switch t.Kind {
	case types.String:
		return "str"
	case types.Int:
		return "int"
	case types.Float:
		return "float"
	case types.Bool:
		return "bool"
	case types.List:
		if t.ElementType != nil {
			return "list[" + typeString(*t.ElementType) + "]"
		}
		return "list"
	case types.Map:
		return "map"
	case types.Any:
		return "any"
	case types.BlockRef:
		return string(t.BlockType)
	case types.Union:
		s := ""
		for i, m := range t.Members {
			if i > 0 {
				s += " | "
			}
			s += typeString(m)
		}
		return s
	default:
		return "unknown"
	}
}

// updateDocument parses the text and stores the document state.
// Always stores the AST, even when it has errors — the parser produces
// a partial AST via error recovery so LSP features still work mid-edit.
func updateDocument(uri, text string) *documentState {
	l := lexer.New(text)
	p := parser.New(l)
	program := p.ParseProgram()

	doc := &documentState{Text: text, Program: program}
	documents[uri] = doc
	return doc
}

// publishDiagnostics sends parse and analyzer diagnostics to the client.
func publishDiagnostics(ctx *glsp.Context, uri string, doc *documentState) {
	diagnostics := Diagnose(doc)
	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}

// Diagnose returns LSP diagnostics for a document. Runs the parser and,
// if parsing succeeds, the analyzer.
// Exported so it can be tested independently of the LSP transport.
func Diagnose(doc *documentState) []protocol.Diagnostic {
	l := lexer.New(doc.Text)
	p := parser.New(l)
	program := p.ParseProgram()

	// Must be an empty slice (not nil) so it marshals to [] in JSON.
	// LSP clients treat null as "no change" but [] as "clear all".
	result := []protocol.Diagnostic{}

	for _, d := range p.Diagnostics() {
		result = append(result, toLspDiagnostic(d))
	}

	// Run semantic analysis only if parsing succeeded without errors.
	if !program.HasErrors {
		for _, d := range analyzer.Analyze(program) {
			result = append(result, toLspDiagnostic(d))
		}
	}

	return result
}

// toLspDiagnostic converts a compiler diagnostic to an LSP protocol diagnostic.
func toLspDiagnostic(d diagnostic.Diagnostic) protocol.Diagnostic {
	severity := toLspSeverity(d.Severity)
	source := d.Source

	start := toLspPosition(d.Position)
	end := start
	if d.EndPosition.Line != 0 || d.EndPosition.Column != 0 {
		end = toLspPosition(d.EndPosition)
	}

	return protocol.Diagnostic{
		Range:    protocol.Range{Start: start, End: end},
		Severity: &severity,
		Source:   &source,
		Message:  d.Message,
	}
}

// toLspPosition converts a 1-based compiler position to a 0-based LSP position.
func toLspPosition(p diagnostic.Position) protocol.Position {
	line := p.Line
	col := p.Column
	if line > 0 {
		line--
	}
	if col > 0 {
		col--
	}
	return protocol.Position{Line: protocol.UInteger(line), Character: protocol.UInteger(col)}
}

// toLspSeverity maps compiler severity levels to LSP severity levels.
func toLspSeverity(s diagnostic.Severity) protocol.DiagnosticSeverity {
	switch s {
	case diagnostic.Error:
		return protocol.DiagnosticSeverityError
	case diagnostic.Warning:
		return protocol.DiagnosticSeverityWarning
	case diagnostic.Info:
		return protocol.DiagnosticSeverityInformation
	case diagnostic.Hint:
		return protocol.DiagnosticSeverityHint
	default:
		return protocol.DiagnosticSeverityError
	}
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool { return &b }

// strPtr returns a pointer to a string value.
func strPtr(s string) *string { return &s }

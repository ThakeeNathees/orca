// Package lsp implements the Language Server Protocol server for Orca.
// It provides real-time diagnostics, autocompletion, and other editor
// features for .oc files.
package lsp

import (
	"fmt"

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
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
)

const serverName = "orca-lsp"

// handler is the LSP protocol handler with all method implementations.
var handler protocol.Handler

// documentState holds the current text, parsed AST, symbol table, and diagnostics
// for an open file. The parser produces a partial AST via error recovery, so
// Program is always set.
type documentState struct {
	Text        string
	Program     *ast.Program
	Symbols     *types.SymbolTable         // symbol table from analysis (nil if parse errors)
	Diagnostics []diagnostic.Diagnostic    // parse + analyzer diagnostics
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
	handler.TextDocumentHover = textDocumentHover
	handler.TextDocumentDefinition = textDocumentDefinition
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
			HoverProvider:      true,
			DefinitionProvider: true,
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{"\n"},
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
		detail := field.Type.String()
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


// textDocumentHover returns type and definition information when the user
// hovers over an identifier, block name, or field name.
func textDocumentHover(ctx *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	doc, ok := documents[params.TextDocument.URI]
	if !ok || doc.Program == nil {
		return nil, nil
	}

	line := int(params.Position.Line) + 1
	col := int(params.Position.Character) + 1

	node := cursor.FindNodeAt(doc.Program, line, col)

	switch node.Kind {
	case cursor.BlockNameNode:
		blockType := token.BlockName(node.Block.TokenStart.Type)
		content := fmt.Sprintf("```orca\n%s %s\n```", blockType, node.Block.Name)
		return &protocol.Hover{
			Contents: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: content},
		}, nil

	case cursor.IdentNode:
		if doc.Symbols == nil {
			return nil, nil
		}
		sym, found := doc.Symbols.LookupSymbol(node.Ident.Value)
		if !found {
			return nil, nil
		}
		content := fmt.Sprintf("```orca\n%s %s\n```", sym.Type.BlockType, node.Ident.Value)
		return &protocol.Hover{
			Contents: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: content},
		}, nil

	case cursor.MemberAccessNode:
		if doc.Symbols == nil {
			return nil, nil
		}
		objType := types.ExprType(node.MemberAccess.Object, doc.Symbols)
		if objType.Kind != types.BlockRef {
			return nil, nil
		}
		field, found := types.GetFieldSchema(string(objType.BlockType), node.MemberAccess.Member)
		if !found {
			return nil, nil
		}
		return fieldHover(node.MemberAccess.Member, field), nil

	case cursor.FieldNameNode:
		blockType := token.BlockName(node.Block.TokenStart.Type)
		field, found := types.GetFieldSchema(blockType, node.Assignment.Name)
		if !found {
			return nil, nil
		}
		return fieldHover(node.Assignment.Name, field), nil
	}

	return nil, nil
}

// fieldHover builds a hover response for a field with its type and description.
func fieldHover(name string, field types.FieldSchema) *protocol.Hover {
	content := fmt.Sprintf("```orca\n%s: %s\n```", name, field.Type.String())
	if field.Description != "" {
		content += "\n\n" + field.Description
	}
	return &protocol.Hover{
		Contents: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: content},
	}
}

// textDocumentDefinition returns the definition location when the user
// triggers go-to-definition on an identifier reference.
func textDocumentDefinition(ctx *glsp.Context, params *protocol.DefinitionParams) (any, error) {
	doc, ok := documents[params.TextDocument.URI]
	if !ok || doc.Program == nil || doc.Symbols == nil {
		return nil, nil
	}

	line := int(params.Position.Line) + 1
	col := int(params.Position.Character) + 1

	node := cursor.FindNodeAt(doc.Program, line, col)

	switch node.Kind {
	case cursor.IdentNode:
		sym, found := doc.Symbols.LookupSymbol(node.Ident.Value)
		if !found {
			return nil, nil
		}
		return protocol.Location{
			URI:   params.TextDocument.URI,
			Range: tokenToRange(sym.DefToken),
		}, nil

	case cursor.MemberAccessNode:
		// Go to the field assignment within the referenced block.
		if ident, ok := node.MemberAccess.Object.(*ast.Identifier); ok {
			block := findBlock(doc.Program, ident.Value)
			if block == nil {
				return nil, nil
			}
			for _, assign := range block.Assignments {
				if assign.Name == node.MemberAccess.Member {
					return protocol.Location{
						URI:   params.TextDocument.URI,
						Range: tokenToRange(assign.Start()),
					}, nil
				}
			}
		}
	}

	return nil, nil
}

// findBlock returns the BlockStatement with the given name, or nil.
func findBlock(program *ast.Program, name string) *ast.BlockStatement {
	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if ok && block.Name == name {
			return block
		}
	}
	return nil
}

// tokenToRange converts a token's position to an LSP Range (0-based).
func tokenToRange(tok token.Token) protocol.Range {
	startLine := tok.Line
	startCol := tok.Column
	if startLine > 0 {
		startLine--
	}
	if startCol > 0 {
		startCol--
	}
	endLine := tok.EndLine
	endCol := tok.EndCol
	if endLine == 0 {
		endLine = tok.Line
	}
	if endCol == 0 {
		endCol = tok.Column + len(tok.Literal)
	}
	if endLine > 0 {
		endLine--
	}
	// endCol is already 1 past the end (exclusive), convert to 0-based.
	if endCol > 0 {
		endCol--
	}
	return protocol.Range{
		Start: protocol.Position{Line: protocol.UInteger(startLine), Character: protocol.UInteger(startCol)},
		End:   protocol.Position{Line: protocol.UInteger(endLine), Character: protocol.UInteger(endCol)},
	}
}

// updateDocument parses the text, runs analysis, and stores everything.
// Called on every open/change — parses once and caches the results.
func updateDocument(uri, text string) *documentState {
	l := lexer.New(text)
	p := parser.New(l)
	program := p.ParseProgram()

	diags := p.Diagnostics()
	var symbols *types.SymbolTable
	if !program.HasErrors {
		result := analyzer.Analyze(program)
		diags = append(diags, result.Diagnostics...)
		symbols = result.Symbols
	}

	doc := &documentState{Text: text, Program: program, Symbols: symbols, Diagnostics: diags}
	documents[uri] = doc
	return doc
}

// publishDiagnostics sends cached diagnostics to the client.
func publishDiagnostics(ctx *glsp.Context, uri string, doc *documentState) {
	// Must be an empty slice (not nil) so it marshals to [] in JSON.
	// LSP clients treat null as "no change" but [] as "clear all".
	lspDiags := make([]protocol.Diagnostic, 0, len(doc.Diagnostics))
	for _, d := range doc.Diagnostics {
		lspDiags = append(lspDiags, toLspDiagnostic(d))
	}
	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: lspDiags,
	})
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

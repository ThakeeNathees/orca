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
				TriggerCharacters: []string{"\n", "."},
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

	// Check for member access completion (triggered by '.').
	// Uses the AST: cursor right after '.' triggers member field completions.
	node := cursor.FindNodeAt(doc.Program, line, col)
	if node.Kind == cursor.MemberAccessNode && node.DotCompletion {
		return completeMemberFields(doc, node.MemberAccess), nil
	}

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

// completeMemberFields returns completion items for an incomplete member access
// expression (e.g. "gpt4." parsed as MemberAccess with empty Member). Resolves
// the object's type through the AST and symbol table, then returns all schema
// fields for that block type. Works with chained access like "foo.bar." by
// recursively resolving the object expression's type.
func completeMemberFields(doc *documentState, ma *ast.MemberAccess) []protocol.CompletionItem {
	if doc.Symbols == nil {
		return nil
	}

	// Resolve the object expression's type through the AST.
	objType := types.ExprType(ma.Object, doc.Symbols)
	if objType.Kind != types.BlockRef {
		return nil
	}

	// Get the schema for that block type.
	schema, ok := types.LookupBlockSchema(objType)
	if !ok {
		return nil
	}

	var items []protocol.CompletionItem
	for name, field := range schema.Fields {
		kind := protocol.CompletionItemKindProperty
		detail := field.Type.String()

		item := protocol.CompletionItem{
			Label:  name,
			Kind:   &kind,
			Detail: &detail,
		}

		if field.Description != "" {
			docContent := protocol.MarkupContent{
				Kind:  protocol.MarkupKindPlainText,
				Value: field.Description,
			}
			item.Documentation = docContent
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
		kind, ok := token.TokenTypeToBlockKind(node.Block.TokenStart.Type)
		if !ok {
			return nil, nil
		}
		content := fmt.Sprintf("```orca\n%s %s\n```", kind.String(), node.Block.Name)
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
		content := fmt.Sprintf("```orca\n%s %s\n```", sym.Type.String(), node.Ident.Value)
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
		field, found := types.LookupFieldSchema(objType, node.MemberAccess.Member)
		if !found {
			return nil, nil
		}
		return fieldHover(node.MemberAccess.Member, field), nil

	case cursor.FieldNameNode:
		kind, ok := token.TokenTypeToBlockKind(node.Block.TokenStart.Type)
		if !ok {
			return nil, nil
		}
		field, found := types.GetFieldSchema(kind, node.Assignment.Name)
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

	loc, found := resolveDefinition(doc, line, col)
	if !found {
		return nil, nil
	}
	loc.URI = params.TextDocument.URI
	return loc, nil
}

// resolveDefinition resolves the definition location for the node at the given
// 1-based position. Returns the location (without URI set) and whether one was found.
// Handles identifiers (jump to block), member access (jump to field assignment
// in the referenced block, using schema to resolve through any block type), and
// field names (jump to the field's assignment in the current block).
func resolveDefinition(doc *documentState, line, col int) (protocol.Location, bool) {
	node := cursor.FindNodeAt(doc.Program, line, col)

	switch node.Kind {
	case cursor.IdentNode:
		sym, found := doc.Symbols.LookupSymbol(node.Ident.Value)
		if !found {
			return protocol.Location{}, false
		}
		return protocol.Location{Range: tokenToRange(sym.DefToken)}, true

	case cursor.MemberAccessNode:
		return resolveMemberDefinition(doc, node.MemberAccess)
	}

	return protocol.Location{}, false
}

// resolveMemberDefinition resolves go-to-definition for a member access expression.
// It resolves the object to either a BlockStatement or SchemaExpression, then
// locates the member's assignment within it. Supports chained access (e.g. a.b.c)
// by recursively resolving intermediate targets.
func resolveMemberDefinition(doc *documentState, ma *ast.MemberAccess) (protocol.Location, bool) {
	target := resolveObjectTarget(doc, ma.Object)

	switch t := target.(type) {
	case *ast.BlockStatement:
		// Look for the field assignment in the block.
		for _, assign := range t.Assignments {
			if assign.Name == ma.Member {
				return protocol.Location{Range: tokenToRange(assign.Start())}, true
			}
		}
		// For input blocks, members access the value schema (the "type" field),
		// not the input block's own fields. e.g. some_input.model_name resolves
		// through input's type = schema { model_name = str }.
		kind, ok := token.TokenTypeToBlockKind(t.TokenStart.Type)
		if ok && kind == token.BlockInput {
			if schema := findTypeSchema(t); schema != nil {
				for _, assign := range schema.Assignments {
					if assign.Name == ma.Member {
						return protocol.Location{Range: tokenToRange(assign.Start())}, true
					}
				}
			}
		}
		// Field not assigned — fall back to the block name token so the user
		// still navigates to the block that owns the schema field.
		if _, ok := types.GetFieldSchema(kind, ma.Member); ok {
			return protocol.Location{Range: tokenToRange(t.NameToken)}, true
		}

	case *ast.SchemaExpression:
		// Look for the field assignment in the inline schema.
		for _, assign := range t.Assignments {
			if assign.Name == ma.Member {
				return protocol.Location{Range: tokenToRange(assign.Start())}, true
			}
		}
	}

	return protocol.Location{}, false
}

// resolveObjectTarget resolves an expression to the AST node that defines its
// members. Returns a *ast.BlockStatement for block references (e.g. gpt4),
// or a *ast.SchemaExpression for inline schemas (e.g. output = schema { ... }).
// For chained access (a.b), recursively resolves the parent then finds the
// member's value within it.
func resolveObjectTarget(doc *documentState, expr ast.Expression) ast.Node {
	switch e := expr.(type) {
	case *ast.Identifier:
		return findBlock(doc.Program, e.Value)

	case *ast.MemberAccess:
		// Resolve the parent object first.
		parent := resolveObjectTarget(doc, e.Object)
		return findMemberValue(parent, e.Member, doc)
	}
	return nil
}

// findMemberValue finds the assignment named `member` within a target node
// (BlockStatement or SchemaExpression) and returns what it resolves to.
// For identifier values, follows the reference to the block. For schema
// expression values, returns the schema itself. Returns nil if not found.
func findMemberValue(target ast.Node, member string, doc *documentState) ast.Node {
	var assignments []*ast.Assignment

	switch t := target.(type) {
	case *ast.BlockStatement:
		assignments = t.Assignments
	case *ast.SchemaExpression:
		assignments = t.Assignments
	default:
		return nil
	}

	for _, assign := range assignments {
		if assign.Name != member {
			continue
		}
		switch v := assign.Value.(type) {
		case *ast.Identifier:
			return findBlock(doc.Program, v.Value)
		case *ast.SchemaExpression:
			return v
		}
		return nil
	}

	// For input blocks, members access the value schema (the "type" field).
	if block, ok := target.(*ast.BlockStatement); ok {
		kind, ok := token.TokenTypeToBlockKind(block.TokenStart.Type)
		if ok && kind == token.BlockInput {
			if schema := findTypeSchema(block); schema != nil {
				for _, assign := range schema.Assignments {
					if assign.Name != member {
						continue
					}
					if se, ok := assign.Value.(*ast.SchemaExpression); ok {
						return se
					}
					return nil
				}
			}
		}
	}

	return nil
}

// findTypeSchema returns the SchemaExpression from the "type" field of a block,
// or nil if not found. Used for input blocks where `type = schema { ... }` defines
// the value schema that members are accessed through.
func findTypeSchema(block *ast.BlockStatement) *ast.SchemaExpression {
	for _, assign := range block.Assignments {
		if assign.Name == "type" {
			if se, ok := assign.Value.(*ast.SchemaExpression); ok {
				return se
			}
			return nil
		}
	}
	return nil
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
	// Always run the analyzer, even with parse errors. The parser produces
	// partial ASTs via error recovery, so we can still resolve symbols for
	// successfully parsed blocks. This enables completions and hover to work
	// while the user is actively typing (which often produces transient errors).
	result := analyzer.Analyze(program)
	if !program.HasErrors {
		diags = append(diags, result.Diagnostics...)
	}
	symbols := result.Symbols

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

	lspDiag := protocol.Diagnostic{
		Range:    protocol.Range{Start: start, End: end},
		Severity: &severity,
		Source:   &source,
		Message:  d.Message,
	}
	if d.Code != "" {
		code := protocol.IntegerOrString{Value: d.Code}
		lspDiag.Code = &code
	}
	return lspDiag
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

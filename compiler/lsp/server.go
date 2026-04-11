// Package lsp implements the Language Server Protocol server for Orca.
// It provides real-time diagnostics, autocompletion, and other editor
// features for .oc files.
package lsp

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
	Symbols     *types.SymbolTable      // symbol table from analysis (nil if parse errors)
	Diagnostics []diagnostic.Diagnostic // parse + analyzer diagnostics
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
	refreshSiblingDiagnostics(ctx, uri)
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
			refreshSiblingDiagnostics(ctx, uri)
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
	// Refresh siblings — removing this file's definitions may introduce
	// or clear diagnostics in other open files.
	refreshSiblingDiagnostics(ctx, uri)
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

	// Collect already-assigned field names from the innermost block.
	assigned := make(map[string]bool)
	if ctx.InlineBlock != nil {
		for _, a := range ctx.InlineBlock.Assignments {
			assigned[a.Name] = true
		}
	} else if ctx.Block != nil {
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
	objType := types.SchemaTypeFromExpr(ma.Object, doc.Symbols)
	if objType.Kind != types.BlockRef {
		return nil
	}

	// Get the schema for that block type.
	if objType.Kind != types.BlockRef || objType.Block == nil {
		return nil
	}
	schema := objType.Block

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
		content := fmt.Sprintf("```orca\n%s %s\n```", node.Block.Kind, node.Block.Name)
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
		objType := types.SchemaTypeFromExpr(node.MemberAccess.Object, doc.Symbols)
		if objType.Kind != types.BlockRef {
			return nil, nil
		}
		field, found := types.LookupFieldSchema(objType, node.MemberAccess.Member)
		if !found {
			return nil, nil
		}
		return fieldHover(node.MemberAccess.Member, field), nil

	case cursor.FieldNameNode:
		field, found := lookupSchemaField(doc.Symbols, node.Block.Kind, node.Assignment.Name)
		if !found {
			return nil, nil
		}
		return fieldHover(node.Assignment.Name, field), nil
	}

	return nil, nil
}

// lookupSchemaField returns the field schema for blockKind.fieldName using the
// analyzed symbol table (block kind → type → schema fields).
func lookupSchemaField(sym *types.SymbolTable, blockKind, fieldName string) (types.FieldSchema, bool) {
	if sym == nil {
		return types.FieldSchema{}, false
	}
	ty, ok := sym.Lookup(blockKind)
	if !ok {
		return types.FieldSchema{}, false
	}
	return types.LookupFieldSchema(ty, fieldName)
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
	// If the definition has a URI set (cross-file), use it; otherwise
	// default to the current document.
	if loc.URI == "" {
		loc.URI = params.TextDocument.URI
	}
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
			// Check if the identifier is a lambda parameter reference.
			if param := findLambdaParam(doc.Program, node.Ident.Value, line, col); param != nil {
				return protocol.Location{Range: tokenToRange(param.Name.TokenStart)}, true
			}
			return protocol.Location{}, false
		}
		loc := protocol.Location{Range: tokenToRange(sym.DefToken)}
		// Check if the definition lives in a sibling file.
		if block := findBlock(doc.Program, node.Ident.Value); block != nil && block.SourceFile != "" {
			loc.URI = "file://" + block.SourceFile
		}
		return loc, true

	case cursor.MemberAccessNode:
		return resolveMemberDefinition(doc, node.MemberAccess)
	}

	return protocol.Location{}, false
}

// findLambdaParam searches for a lambda parameter with the given name that
// encloses the specified position. Returns the matching LambdaParam if found.
func findLambdaParam(program *ast.Program, name string, line, col int) *ast.LambdaParam {
	if program == nil {
		return nil
	}
	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		for _, assign := range block.Assignments {
			if p := findLambdaParamInExpr(assign.Value, name, line, col); p != nil {
				return p
			}
		}
		for _, expr := range block.Expressions {
			if p := findLambdaParamInExpr(expr, name, line, col); p != nil {
				return p
			}
		}
	}
	return nil
}

// findLambdaParamInExpr recursively searches for a lambda that encloses the
// given position and has a parameter matching name. Returns the innermost match.
func findLambdaParamInExpr(expr ast.Expression, name string, line, col int) *ast.LambdaParam {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.Lambda:
		// Check if position is inside this lambda's body.
		if p := findLambdaParamInExpr(e.Body, name, line, col); p != nil {
			return p // inner lambda shadows
		}
		// Check if this lambda has a matching param and the position is within the lambda.
		start, end := e.Start(), e.End()
		if posAfterOrAt(line, col, start.Line, start.Column) &&
			posBeforeOrAt(line, col, end.Line, end.Column) {
			for i := range e.Params {
				if e.Params[i].Name.Value == name {
					return &e.Params[i]
				}
			}
		}
	case *ast.BinaryExpression:
		if p := findLambdaParamInExpr(e.Left, name, line, col); p != nil {
			return p
		}
		if p := findLambdaParamInExpr(e.Right, name, line, col); p != nil {
			return p
		}
	case *ast.CallExpression:
		if p := findLambdaParamInExpr(e.Callee, name, line, col); p != nil {
			return p
		}
		for _, arg := range e.Arguments {
			if p := findLambdaParamInExpr(arg, name, line, col); p != nil {
				return p
			}
		}
	case *ast.MemberAccess:
		return findLambdaParamInExpr(e.Object, name, line, col)
	case *ast.Subscription:
		if p := findLambdaParamInExpr(e.Object, name, line, col); p != nil {
			return p
		}
		for _, idx := range e.Indices {
			if p := findLambdaParamInExpr(idx, name, line, col); p != nil {
				return p
			}
		}
	case *ast.TernaryExpression:
		if p := findLambdaParamInExpr(e.Condition, name, line, col); p != nil {
			return p
		}
		if p := findLambdaParamInExpr(e.TrueExpr, name, line, col); p != nil {
			return p
		}
		return findLambdaParamInExpr(e.FalseExpr, name, line, col)
	case *ast.ListLiteral:
		for _, el := range e.Elements {
			if p := findLambdaParamInExpr(el, name, line, col); p != nil {
				return p
			}
		}
	case *ast.MapLiteral:
		for _, entry := range e.Entries {
			if p := findLambdaParamInExpr(entry.Key, name, line, col); p != nil {
				return p
			}
			if p := findLambdaParamInExpr(entry.Value, name, line, col); p != nil {
				return p
			}
		}
	case *ast.BlockExpression:
		if e == nil {
			return nil
		}
		for _, assign := range e.Assignments {
			if p := findLambdaParamInExpr(assign.Value, name, line, col); p != nil {
				return p
			}
		}
		for _, expr := range e.Expressions {
			if p := findLambdaParamInExpr(expr, name, line, col); p != nil {
				return p
			}
		}
	}
	return nil
}

// posAfterOrAt returns true if (line, col) is at or after (startLine, startCol).
func posAfterOrAt(line, col, startLine, startCol int) bool {
	return line > startLine || (line == startLine && col >= startCol)
}

// posBeforeOrAt returns true if (line, col) is at or before (endLine, endCol).
func posBeforeOrAt(line, col, endLine, endCol int) bool {
	return line < endLine || (line == endLine && col <= endCol)
}

// resolveMemberDefinition resolves go-to-definition for a member access expression.
// It resolves the object to either a BlockStatement or BlockExpression, then
// locates the member's assignment within it. Supports chained access (e.g. a.b.c)
// by recursively resolving intermediate targets.
func resolveMemberDefinition(doc *documentState, ma *ast.MemberAccess) (protocol.Location, bool) {
	target := resolveObjectTarget(doc, ma.Object)

	// setURI populates the cross-file URI when the target block lives in a
	// sibling source file.
	setURI := func(loc *protocol.Location) {
		if block, ok := target.(*ast.BlockStatement); ok && block.SourceFile != "" {
			loc.URI = "file://" + block.SourceFile
		}
	}

	if loc, ok := findAssignmentLocation(target, ma.Member); ok {
		setURI(&loc)
		return loc, true
	}

	if block, ok := target.(*ast.BlockStatement); ok {
		if _, ok := lookupSchemaField(doc.Symbols, block.Kind, ma.Member); ok {
			// Prefer the field's definition in a user `schema <kind>` block in this
			// program. If the schema only exists in bootstrap (e.g. model), keep
			// jumping to the instance block name (e.g. gpt4) — see
			// TestDefinitionMemberUnassignedField.
			if loc, ok := findSchemaFieldAssignmentInProgram(doc.Program, block.Kind, ma.Member); ok {
				setURI(&loc)
				return loc, true
			}
			loc := protocol.Location{Range: tokenToRange(block.NameToken)}
			setURI(&loc)
			return loc, true
		}
	}

	return protocol.Location{}, false
}

// findAssignmentLocation searches a node's assignments for the given field
// name and returns the location of the matching assignment token.
func findAssignmentLocation(node ast.Node, field string) (protocol.Location, bool) {
	for _, assign := range assignmentsOf(node) {
		if assign.Name == field {
			return protocol.Location{Range: tokenToRange(assign.Start())}, true
		}
	}
	return protocol.Location{}, false
}

// assignmentsOf extracts the Assignments slice from any node that embeds
// BlockBody (BlockStatement or BlockExpression).
func assignmentsOf(node ast.Node) []*ast.Assignment {
	switch t := node.(type) {
	case *ast.BlockStatement:
		return t.Assignments
	case *ast.BlockExpression:
		return t.Assignments
	}
	return nil
}

// resolveObjectTarget resolves an expression to the AST node that defines its
// members. Returns a *ast.BlockStatement for block references (e.g. gpt4),
// or a *ast.BlockExpression for inline blocks (e.g. output = schema { ... }).
// For chained access (a.b), recursively resolves the parent then finds the
// member's value within it. For list[index].field, subscription unwraps to the
// element schema (homogeneous lists).
func resolveObjectTarget(doc *documentState, expr ast.Expression) ast.Node {
	switch e := expr.(type) {
	case *ast.Identifier:
		return findBlock(doc.Program, e.Value)

	case *ast.MemberAccess:
		// Resolve the parent object first.
		parent := resolveObjectTarget(doc, e.Object)
		return findMemberValue(parent, e.Member, doc)

	case *ast.Subscription:
		base := resolveObjectTarget(doc, e.Object)
		if be, ok := base.(*ast.BlockExpression); ok {
			return be
		}
		return nil
	}
	return nil
}

// findMemberValue finds the assignment named `member` within a target node
// (BlockStatement or BlockExpression) and returns what it resolves to.
// For identifier values, follows the reference to the block. For block
// expression values, returns the expression itself. If the instance block has
// no assignment for the field, falls back to the type's `schema <kind> { ... }`
// definition (user program or symbol table). Returns nil if not found.
func findMemberValue(target ast.Node, member string, doc *documentState) ast.Node {
	if result := resolveAssignmentValue(assignmentsOf(target), member, doc); result != nil {
		return result
	}
	if block, ok := target.(*ast.BlockStatement); ok {
		return resolveAssignmentValue(schemaKindAssignments(doc, block.Kind), member, doc)
	}
	return nil
}

// resolveAssignmentValue finds the assignment named `member` in the given
// assignments and resolves its value: identifiers follow to blocks, inline
// blocks return the expression itself, and list [ schema { ... } ] unwraps to
// the inner schema block expression.
func resolveAssignmentValue(assignments []*ast.Assignment, member string, doc *documentState) ast.Node {
	for _, assign := range assignments {
		if assign.Name != member {
			continue
		}
		return resolveExprValue(assign.Value, doc)
	}
	return nil
}

// resolveExprValue maps a field value expression to the AST used for further
// member resolution (block ref, inline schema, or list element schema).
func resolveExprValue(expr ast.Expression, doc *documentState) ast.Node {
	if expr == nil {
		return nil
	}
	switch v := expr.(type) {
	case *ast.Identifier:
		return findBlock(doc.Program, v.Value)
	case *ast.BlockExpression:
		return v
	case *ast.Subscription:
		return resolveListSchemaElement(v)
	default:
		return nil
	}
}

// resolveListSchemaElement returns the inner schema block for `list [ schema { ... } ]`.
func resolveListSchemaElement(s *ast.Subscription) ast.Node {
	if id, ok := s.Object.(*ast.Identifier); ok && id.Value == "list" && len(s.Indices) > 0 {
		if inner, ok := s.Indices[0].(*ast.BlockExpression); ok {
			return inner
		}
	}
	return nil
}

// findSchemaBlockStatement returns the top-level `schema typeName { ... }` block
// in the program, or nil if this file does not define that schema.
func findSchemaBlockStatement(program *ast.Program, typeName string) *ast.BlockStatement {
	if program == nil {
		return nil
	}
	for _, stmt := range program.Statements {
		b, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		if b.Kind == "schema" && b.Name == typeName {
			return b
		}
	}
	return nil
}

// findSchemaFieldAssignmentInProgram returns the definition location for a field
// declared in a user-visible `schema <kind>` block in this program.
func findSchemaFieldAssignmentInProgram(program *ast.Program, kindName, fieldName string) (protocol.Location, bool) {
	sb := findSchemaBlockStatement(program, kindName)
	if sb == nil {
		return protocol.Location{}, false
	}
	for _, assign := range sb.Assignments {
		if assign.Name == fieldName {
			return protocol.Location{Range: tokenToRange(assign.Start())}, true
		}
	}
	return protocol.Location{}, false
}

// schemaKindAssignments returns assignments from the schema that defines kindName:
// first from a `schema kindName` block in the program, otherwise from the
// analyzed symbol table (bootstrap / merged definitions).
func schemaKindAssignments(doc *documentState, kindName string) []*ast.Assignment {
	if doc.Program != nil {
		if sb := findSchemaBlockStatement(doc.Program, kindName); sb != nil {
			return sb.Assignments
		}
	}
	if doc.Symbols == nil {
		return nil
	}
	ty, ok := doc.Symbols.Lookup(kindName)
	if !ok || ty.Block == nil || ty.Block.Ast == nil {
		return nil
	}
	return ty.Block.Ast.Assignments
}

// findTypeSchema returns the BlockExpression from the "type" field of a block,
// or nil if not found. Used for input blocks where `type = schema { ... }` defines
// the value schema that members are accessed through.
func findTypeSchema(block *ast.BlockStatement) *ast.BlockExpression {
	expr, ok := block.GetFieldExpression("type")
	if !ok {
		return nil
	}
	be, _ := expr.(*ast.BlockExpression)
	return be
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
//
// To support cross-file references (e.g. an input block in inputs.oc
// referenced from main.oc), we build a combined program from all .oc
// files in the same directory. The current file's text comes from the
// editor buffer (may have unsaved changes); sibling files are read from
// disk. Analysis runs on the merged program, but only diagnostics that
// originate from the current file are reported to the client.
func updateDocument(uri, text string) *documentState {
	l := lexer.New(text, "")
	p := parser.New(l)
	program := p.ParseProgram()

	diags := p.Diagnostics()
	filePath := uriToPath(uri)

	// Merge sibling .oc files so cross-file symbols are visible.
	mergeSiblingFiles(uri, program)

	// Always run the analyzer, even with parse errors. The parser produces
	// partial ASTs via error recovery, so we can still resolve symbols for
	// successfully parsed blocks. This enables completions and hover to work
	// while the user is actively typing (which often produces transient errors).
	result := analyzer.Analyze(program)
	if !program.HasErrors {
		// Only keep diagnostics from the current file. Diagnostics with
		// an empty File originate from the current file (no SourceFile
		// set); diagnostics with a File set come from sibling files.
		for _, d := range result.Diagnostics {
			if d.File == "" || d.File == filePath {
				diags = append(diags, d)
			}
		}
	}
	symbols := result.SymbolTable

	doc := &documentState{Text: text, Program: program, Symbols: symbols, Diagnostics: diags}
	documents[uri] = doc
	return doc
}

// mergeSiblingFiles finds all .oc files in the same directory as uri,
// parses them, and appends their statements to program. Skips the file
// identified by uri itself (already parsed from the editor buffer).
func mergeSiblingFiles(uri string, program *ast.Program) {
	filePath := uriToPath(uri)
	if filePath == "" {
		return
	}
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)

	siblings, err := filepath.Glob(filepath.Join(dir, "*.oc"))
	if err != nil {
		return
	}

	for _, sib := range siblings {
		if filepath.Base(sib) == base {
			continue
		}

		// Prefer the in-memory buffer if the file is already open in the
		// editor — it may contain unsaved changes not yet written to disk.
		sibURI := "file://" + sib
		var source string
		if doc, ok := documents[sibURI]; ok {
			source = doc.Text
		} else {
			data, err := os.ReadFile(sib)
			if err != nil {
				continue
			}
			source = string(data)
		}

		sl := lexer.New(source, sib)
		sp := parser.New(sl)
		sibProg := sp.ParseProgram()
		program.Statements = append(program.Statements, sibProg.Statements...)
	}
}

// uriToPath converts a file:// URI to a filesystem path.
func uriToPath(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return ""
	}
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	return u.Path
}

// refreshSiblingDiagnostics re-analyzes and republishes diagnostics for all
// open documents that are siblings of changedURI. When one file changes, its
// edits can affect diagnostics in other files (e.g. adding/removing a model
// that siblings reference), so we must refresh them all.
func refreshSiblingDiagnostics(ctx *glsp.Context, changedURI string) {
	changedPath := uriToPath(changedURI)
	if changedPath == "" {
		return
	}
	dir := filepath.Dir(changedPath)

	for uri, doc := range documents {
		if uri == changedURI {
			continue
		}
		sibPath := uriToPath(uri)
		if sibPath == "" || filepath.Dir(sibPath) != dir {
			continue
		}
		// Re-analyze sibling with its current buffer text.
		updated := updateDocument(uri, doc.Text)
		publishDiagnostics(ctx, uri, updated)
	}
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

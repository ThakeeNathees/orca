// Package cursor resolves the semantic context at a given source position.
// Used by LSP features (completion, hover, go-to-definition) to understand
// what the cursor is pointing at without re-implementing position logic
// in every handler.
package cursor

import (
	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
)

// CursorPosition describes where the cursor sits relative to the language structure.
type CursorPosition int

const (
	// TopLevel means the cursor is outside any block.
	TopLevel CursorPosition = iota
	// BlockBody means the cursor is inside a block body where a field name is expected.
	BlockBody
	// FieldValue means the cursor is on the value side of an assignment (after '=').
	FieldValue
)

// Context holds the resolved semantic context at a cursor position.
// Each LSP feature reads the fields it needs without duplicating lookup logic.
type Context struct {
	Position    CursorPosition      // where the cursor sits structurally
	Block       *ast.BlockStatement // enclosing top-level block, nil if TopLevel
	InlineBlock *ast.BlockBody      // innermost block body (inline), nil if not inside one
	BlockKind   string              // block kind (of the innermost block)
	Schema      *types.BlockSchema  // schema for the block type, nil if unknown
	Assignment  *ast.Assignment     // enclosing assignment, nil if not on a value
}

// Resolve determines the semantic context at the given 1-based line and column
// within the program's AST.
func Resolve(program *ast.Program, line, col int) Context {
	if program == nil {
		return Context{Position: TopLevel}
	}

	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}

		if !posInBlock(block, line, col) {
			continue
		}

		ctx := Context{
			Position:  BlockBody,
			Block:     block,
			BlockKind: block.Kind,
		}
		ctx.Schema = resolveBlockSchema(block.Kind, block.Name)

		// Check if the cursor is within an existing assignment's range.
		// Uses token EndLine/EndCol for multi-line tokens (strings).
		for _, assign := range block.Assignments {
			if posInAssignment(assign, line, col) {
				// Check if the value is an inline block and cursor is inside it.
				if be, ok := assign.Value.(*ast.BlockExpression); ok {
					if inlineCtx, found := resolveInlineBlock(be, block, line, col); found {
						return inlineCtx
					}
				}
				ctx.Position = FieldValue
				ctx.Assignment = assign
				break
			}
		}

		return ctx
	}

	return Context{Position: TopLevel}
}

// resolveInlineBlock checks if the cursor is inside a BlockExpression's body
// and returns the appropriate context. Returns (ctx, true) if inside, or
// (Context{}, false) if the cursor is not within the inline block body.
func resolveInlineBlock(be *ast.BlockExpression, parent *ast.BlockStatement, line, col int) (Context, bool) {
	// Check if cursor is within the inline block's braces.
	startLine := be.TokenStart.Line
	startCol := be.TokenStart.Column
	endLine := be.TokenEnd.Line
	endCol := be.TokenEnd.Column
	if !posAfterOrAt(line, col, startLine, startCol) || !posBeforeOrAt(line, col, endLine, endCol) {
		return Context{}, false
	}

	ctx := Context{
		Position:    BlockBody,
		Block:       parent,
		InlineBlock: &be.BlockBody,
		BlockKind:   be.Kind,
	}
	// Inline schemas are anonymous — pass empty name so resolveBlockSchema
	// skips the named-schema lookup and returns nil.
	ctx.Schema = resolveBlockSchema(be.Kind, "")

	// Check if cursor is within an assignment inside the inline block.
	for _, assign := range be.Assignments {
		if posInAssignment(assign, line, col) {
			ctx.Position = FieldValue
			ctx.Assignment = assign
			break
		}
	}

	return ctx, true
}

// resolveBlockSchema returns the schema for a block kind. For user-defined
// schema blocks, uses the block name as the lookup key (e.g. "vpc_data_t");
// for all other kinds, uses the kind string (e.g. "model"). Returns nil when
// no schema is registered (e.g. anonymous inline schemas with empty name).
func resolveBlockSchema(kind string, name string) *types.BlockSchema {
	var schemaName string
	if kind == types.BlockKindSchema && name != "" {
		schemaName = name
	} else {
		schemaName = kind
	}
	if schema, ok := types.GetSchema(schemaName); ok {
		return &schema
	}
	return nil
}

// posInAssignment returns true if (line, col) falls within an assignment's
// full range, using EndLine/EndCol from the end token for multi-line values.
func posInAssignment(assign *ast.Assignment, line, col int) bool {
	start := assign.Start()
	end := assign.End()
	return posAfterOrAt(line, col, start.Line, start.Column) &&
		posBeforeOrAt(line, col, end.EndLine, end.EndCol)
}

// posInBlock returns true if (line, col) falls within the block's body,
// between the opening '{' and closing '}' inclusive.
func posInBlock(block *ast.BlockStatement, line, col int) bool {
	startLine := block.OpenBrace.Line
	startCol := block.OpenBrace.Column
	endLine := block.TokenEnd.Line
	endCol := block.TokenEnd.Column

	return posAfterOrAt(line, col, startLine, startCol) &&
		posBeforeOrAt(line, col, endLine, endCol)
}

// NodeKind classifies the kind of AST element found at a cursor position.
type NodeKind int

const (
	// NoneNode means no relevant node was found at the position.
	NoneNode NodeKind = iota
	// BlockNameNode means the cursor is on a block's name token (definition site).
	BlockNameNode
	// IdentNode means the cursor is on an identifier reference.
	IdentNode
	// MemberAccessNode means the cursor is on a member access expression.
	MemberAccessNode
	// FieldNameNode means the cursor is on the key (left side) of an assignment.
	FieldNameNode
)

// NodeAt describes which AST element the cursor is pointing at.
type NodeAt struct {
	Kind          NodeKind
	Block         *ast.BlockStatement // enclosing block (always set if not NoneNode)
	Ident         *ast.Identifier     // set for IdentNode
	MemberAccess  *ast.MemberAccess   // set for MemberAccessNode
	Assignment    *ast.Assignment     // set for FieldNameNode
	DotCompletion bool                // true when cursor is right after '.' (needs member completions)
}

// FindNodeAt returns the AST element at the given 1-based line and column.
// It checks block names, field names, and expressions within assignments.
func FindNodeAt(program *ast.Program, line, col int) NodeAt {
	if program == nil {
		return NodeAt{}
	}

	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}

		// Check if cursor is on the block name token.
		if posOnToken(block.NameToken, line, col) {
			return NodeAt{Kind: BlockNameNode, Block: block}
		}

		if !posInBlock(block, line, col) {
			continue
		}

		// Check assignments.
		for _, assign := range block.Assignments {
			// Check if cursor is on the field name (key).
			keyTok := assign.Start()
			if posOnToken(keyTok, line, col) {
				return NodeAt{Kind: FieldNameNode, Block: block, Assignment: assign}
			}

			// Check expressions in the value.
			if node := findInExpr(assign.Value, block, line, col); node.Kind != NoneNode {
				return node
			}
		}

		// Check bare expressions (e.g. workflow edge chains A -> B -> C).
		for _, expr := range block.Expressions {
			if node := findInExpr(expr, block, line, col); node.Kind != NoneNode {
				return node
			}
		}
	}

	return NodeAt{}
}

// findInExpr recursively searches an expression tree for the deepest node
// at the given position. Prefers more specific nodes (identifiers inside
// member access) over broader ones.
func findInExpr(expr ast.Expression, block *ast.BlockStatement, line, col int) NodeAt {
	if expr == nil {
		return NodeAt{}
	}

	switch e := expr.(type) {
	case *ast.BlockExpression:
		// Recurse into inline block assignments and expressions.
		for _, assign := range e.Assignments {
			keyTok := assign.Start()
			if posOnToken(keyTok, line, col) {
				return NodeAt{Kind: FieldNameNode, Block: block, Assignment: assign}
			}
			if node := findInExpr(assign.Value, block, line, col); node.Kind != NoneNode {
				return node
			}
		}
		for _, expr := range e.Expressions {
			if node := findInExpr(expr, block, line, col); node.Kind != NoneNode {
				return node
			}
		}
	case *ast.Identifier:
		if posOnToken(e.Start(), line, col) {
			return NodeAt{Kind: IdentNode, Block: block, Ident: e}
		}
	case *ast.MemberAccess:
		// Check if cursor is right after the dot (dot-completion position).
		// This covers both incomplete "gpt4." (empty Member) and recovered
		// "gpt4.persona" where persona was on the next line.
		dot := e.Dot
		afterDotCol := dot.Column + len(dot.Literal)
		if line == dot.Line && col == afterDotCol {
			return NodeAt{Kind: MemberAccessNode, Block: block, MemberAccess: e, DotCompletion: true}
		}
		if e.Member != "" {
			// Check the member name token (the end token).
			if posOnToken(e.End(), line, col) {
				return NodeAt{Kind: MemberAccessNode, Block: block, MemberAccess: e}
			}
		}
		// Check the object side.
		if node := findInExpr(e.Object, block, line, col); node.Kind != NoneNode {
			return node
		}
	case *ast.ListLiteral:
		for _, elem := range e.Elements {
			if node := findInExpr(elem, block, line, col); node.Kind != NoneNode {
				return node
			}
		}
	case *ast.BinaryExpression:
		if node := findInExpr(e.Left, block, line, col); node.Kind != NoneNode {
			return node
		}
		if node := findInExpr(e.Right, block, line, col); node.Kind != NoneNode {
			return node
		}
	case *ast.CallExpression:
		if node := findInExpr(e.Callee, block, line, col); node.Kind != NoneNode {
			return node
		}
		for _, arg := range e.Arguments {
			if node := findInExpr(arg, block, line, col); node.Kind != NoneNode {
				return node
			}
		}
	case *ast.Subscription:
		if node := findInExpr(e.Object, block, line, col); node.Kind != NoneNode {
			return node
		}
		if node := findInExpr(e.Index, block, line, col); node.Kind != NoneNode {
			return node
		}
	case *ast.MapLiteral:
		for _, entry := range e.Entries {
			if node := findInExpr(entry.Key, block, line, col); node.Kind != NoneNode {
				return node
			}
			if node := findInExpr(entry.Value, block, line, col); node.Kind != NoneNode {
				return node
			}
		}
	}

	return NodeAt{}
}

// posOnToken returns true if (line, col) falls within the token's span.
func posOnToken(tok token.Token, line, col int) bool {
	startLine, startCol := tok.Line, tok.Column
	endLine, endCol := tok.EndLine, tok.EndCol
	if endLine == 0 {
		endLine = startLine
	}
	if endCol == 0 {
		endCol = startCol + len(tok.Literal) - 1
	}
	return posAfterOrAt(line, col, startLine, startCol) &&
		posBeforeOrAt(line, col, endLine, endCol)
}

// posAfterOrAt returns true if (line, col) is at or after (refLine, refCol).
func posAfterOrAt(line, col, refLine, refCol int) bool {
	return line > refLine || (line == refLine && col >= refCol)
}

// posBeforeOrAt returns true if (line, col) is at or before (refLine, refCol).
func posBeforeOrAt(line, col, refLine, refCol int) bool {
	return line < refLine || (line == refLine && col <= refCol)
}

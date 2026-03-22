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
	Position   CursorPosition      // where the cursor sits structurally
	Block      *ast.BlockStatement // enclosing block, nil if TopLevel
	BlockType  string              // lowercase block type name (e.g. "model")
	Schema     *types.BlockSchema  // schema for the block type, nil if unknown
	Assignment *ast.Assignment     // enclosing assignment, nil if not on a value
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

		blockType := token.BlockName(block.TokenStart.Type)
		ctx := Context{
			Position:  BlockBody,
			Block:     block,
			BlockType: blockType,
		}

		// Attach schema if available.
		if schema, ok := types.GetBlockSchema(blockType); ok {
			ctx.Schema = &schema
		}

		// Check if the cursor is within an existing assignment's range.
		// Uses token EndLine/EndCol for multi-line tokens (strings).
		for _, assign := range block.Assignments {
			if posInAssignment(assign, line, col) {
				ctx.Position = FieldValue
				ctx.Assignment = assign
				break
			}
		}

		return ctx
	}

	return Context{Position: TopLevel}
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

// posAfterOrAt returns true if (line, col) is at or after (refLine, refCol).
func posAfterOrAt(line, col, refLine, refCol int) bool {
	return line > refLine || (line == refLine && col >= refCol)
}

// posBeforeOrAt returns true if (line, col) is at or before (refLine, refCol).
func posBeforeOrAt(line, col, refLine, refCol int) bool {
	return line < refLine || (line == refLine && col <= refCol)
}

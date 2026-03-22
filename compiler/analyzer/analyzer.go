// Package analyzer performs semantic analysis on the AST produced by the parser.
// It resolves references between blocks (e.g., verifying that an agent's model
// refers to a defined model block), checks for type mismatches, undefined
// identifiers, and other errors that can't be caught by syntax alone.
package analyzer

import (
	"fmt"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
)

// Analyze walks the AST and performs semantic analysis.
// Builds a symbol table from all block definitions, then validates
// each block's fields against its schema.
func Analyze(program *ast.Program) []diagnostic.Diagnostic {
	symbols := buildSymbolTable(program)
	var diags []diagnostic.Diagnostic

	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		diags = append(diags, analyzeBlock(block, symbols)...)
	}

	return diags
}

// buildSymbolTable walks all block statements and registers each block
// name with its block reference type, so identifiers can be resolved.
func buildSymbolTable(program *ast.Program) *types.SymbolTable {
	st := types.NewSymbolTable()
	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		blockType := token.BlockName(block.TokenStart.Type)
		kind, ok := types.BlockKindFromName(blockType)
		if !ok {
			continue
		}
		st.Define(block.Name, types.NewBlockRefType(kind))
	}
	return st
}

// analyzeBlock performs all validation checks on a single block statement.
// Looks up the block's schema and validates each field, then checks for
// missing required fields.
func analyzeBlock(block *ast.BlockStatement, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	typeName := token.BlockName(block.TokenStart.Type)
	schema, ok := types.GetBlockSchema(typeName)
	if !ok {
		return nil
	}

	var diags []diagnostic.Diagnostic

	// Validate each field present in the block.
	present := make(map[string]bool, len(block.Assignments))
	for _, assign := range block.Assignments {
		present[assign.Name] = true
		diags = append(diags, validateField(block, assign, schema, symbols)...)
	}

	// Check for missing required fields.
	for fieldName, fieldSchema := range schema.Fields {
		if fieldSchema.Required && !present[fieldName] {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Position: diagnostic.Position{
					Line:   block.OpenBrace.Line,
					Column: block.OpenBrace.Column,
				},
				EndPosition: diagnostic.Position{
					Line:   block.TokenEnd.Line,
					Column: block.TokenEnd.Column + 1,
				},
				Message: fmt.Sprintf("block %q is missing required field %q", block.Name, fieldName),
				Source:  "analyzer",
			})
		}
	}

	return diags
}

// validateField checks a single field assignment against the block's schema.
// Verifies the field's value type matches the schema's expected type.
// Only reports mismatches when the expression type is concrete (not Any),
// since identifiers and complex expressions aren't resolved yet.
func validateField(block *ast.BlockStatement, assign *ast.Assignment, schema types.BlockSchema, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	fieldSchema, ok := schema.Fields[assign.Name]
	if !ok {
		// Unknown field — could report a warning, but skip for now.
		// TODO: report unknown field names.
		return nil
	}

	exprType := types.ExprType(assign.Value, symbols)
	// Skip validation when the expression type is unknown (identifiers,
	// complex expressions). These need scope resolution first.
	if exprType.Kind == types.Any {
		return nil
	}

	expected := fieldSchema.Type
	if typeMatches(exprType, expected) {
		return nil
	}

	return []diagnostic.Diagnostic{{
		Severity: diagnostic.Error,
		Position: diagnostic.Position{
			Line:   assign.Value.Start().Line,
			Column: assign.Value.Start().Column,
		},
		Message: fmt.Sprintf("field %q expects type %s, got %s",
			assign.Name, expected.String(), exprType.String()),
		Source: "analyzer",
	}}
}

// typeMatches returns true if the expression type is compatible with the
// expected schema type. Handles unions (expr matches if it matches any member)
// and Any (always matches).
func typeMatches(expr, expected types.Type) bool {
	if expected.Kind == types.Any {
		return true
	}
	if expected.Kind == types.Union {
		return expected.Contains(expr)
	}
	// For lists, match if kinds match (ignore element type for now).
	if expected.Kind == types.List && expr.Kind == types.List {
		return true
	}
	return expr.Kind == expected.Kind
}

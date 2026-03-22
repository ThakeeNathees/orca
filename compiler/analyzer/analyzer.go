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
	var diags []diagnostic.Diagnostic

	symbols, dupDiags := buildSymbolTable(program)
	diags = append(diags, dupDiags...)

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
// name with its block reference type. Reports duplicate block names.
func buildSymbolTable(program *ast.Program) (*types.SymbolTable, []diagnostic.Diagnostic) {
	st := types.NewSymbolTable()
	var diags []diagnostic.Diagnostic

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

		if _, exists := st.Lookup(block.Name); exists {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Position: diagnostic.Position{
					Line:   block.TokenStart.Line,
					Column: block.TokenStart.Column,
				},
				Message: fmt.Sprintf("duplicate block name %q", block.Name),
				Source:  "analyzer",
			})
		}
		st.Define(block.Name, types.NewBlockRefType(kind))
	}
	return st, diags
}

// analyzeBlock performs all validation checks on a single block statement.
// Checks for duplicate fields, unknown fields, missing required fields,
// undefined references, and type mismatches.
func analyzeBlock(block *ast.BlockStatement, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	typeName := token.BlockName(block.TokenStart.Type)
	schema, ok := types.GetBlockSchema(typeName)
	if !ok {
		return nil
	}

	var diags []diagnostic.Diagnostic

	// Validate each field present in the block.
	seen := make(map[string]bool, len(block.Assignments))
	for _, assign := range block.Assignments {
		// Check for duplicate field names.
		if seen[assign.Name] {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Position: diagnostic.Position{
					Line:   assign.Start().Line,
					Column: assign.Start().Column,
				},
				Message: fmt.Sprintf("duplicate field %q in %s %q", assign.Name, typeName, block.Name),
				Source:  "analyzer",
			})
		}
		seen[assign.Name] = true
		diags = append(diags, validateField(block, assign, schema, symbols)...)
	}

	// Check for missing required fields.
	for fieldName, fieldSchema := range schema.Fields {
		if fieldSchema.Required && !seen[fieldName] {
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
// Reports unknown fields, undefined identifier references, and type mismatches.
func validateField(block *ast.BlockStatement, assign *ast.Assignment, schema types.BlockSchema, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	fieldSchema, ok := schema.Fields[assign.Name]
	if !ok {
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Position: diagnostic.Position{
				Line:   assign.Start().Line,
				Column: assign.Start().Column,
			},
			Message: fmt.Sprintf("unknown field %q in %s block", assign.Name, token.BlockName(block.TokenStart.Type)),
			Source:  "analyzer",
		}}
	}

	// Check for undefined identifier references.
	if ident, ok := assign.Value.(*ast.Identifier); ok {
		if _, found := symbols.Lookup(ident.Value); !found {
			return []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Position: diagnostic.Position{
					Line:   ident.Start().Line,
					Column: ident.Start().Column,
				},
				Message: fmt.Sprintf("undefined reference %q", ident.Value),
				Source:  "analyzer",
			}}
		}
	}

	exprType := types.ExprType(assign.Value, symbols)
	// Skip type validation when the expression type is unknown.
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

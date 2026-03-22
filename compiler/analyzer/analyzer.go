// Package analyzer performs semantic analysis on the AST produced by the parser.
// It resolves references between blocks (e.g., verifying that an agent's model
// refers to a defined model block), checks for type mismatches, undefined
// identifiers, and other errors that can't be caught by syntax alone.
package analyzer

import (
	"fmt"
	"strings"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/types"
)

// Analyze walks the AST and performs semantic analysis.
// Returns a list of diagnostics found during analysis.
func Analyze(program *ast.Program) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic

	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		diags = append(diags, analyzeBlock(block)...)
	}

	return diags
}

// analyzeBlock performs all validation checks on a single block statement.
// Looks up the block's schema and validates each field, then checks for
// missing required fields.
func analyzeBlock(block *ast.BlockStatement) []diagnostic.Diagnostic {
	// TokenType is an uppercase string (e.g. "MODEL"); schema keys are lowercase.
	typeName := strings.ToLower(string(block.TokenStart.Type))
	schema, ok := types.GetBlockSchema(typeName)
	if !ok {
		return nil
	}

	var diags []diagnostic.Diagnostic

	// Validate each field present in the block.
	present := make(map[string]bool, len(block.Assignments))
	for _, assign := range block.Assignments {
		present[assign.Name] = true
		diags = append(diags, validateField(block, assign, schema)...)
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
func validateField(block *ast.BlockStatement, assign *ast.Assignment, schema types.BlockSchema) []diagnostic.Diagnostic {
	// TODO: validate that the field's value type matches the schema type.
	return nil
}

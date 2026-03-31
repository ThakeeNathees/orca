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

// AnalyzedProgram holds the output of semantic analysis: the symbol table
// built from block definitions and any diagnostics produced.
type AnalyzedProgram struct {
	Ast         *ast.Program
	SymbolTable *types.SymbolTable
	Diagnostics []diagnostic.Diagnostic
}

// Analyze walks the AST and performs semantic analysis.
// Builds a symbol table from all block definitions, then validates
// each block's fields against its schema. Returns the symbol table
// along with diagnostics so callers (like the LSP) can use it for
// hover, go-to-definition, and other features.
func Analyze(program *ast.Program) AnalyzedProgram {
	var diags []diagnostic.Diagnostic

	symbols, dupDiags := buildSymbolTable(program)
	registerUserSchemas(program)
	diags = append(diags, dupDiags...)

	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}

		var blockDiags []diagnostic.Diagnostic
		// Let blocks are free-form variable bindings with no schema, so
		// they can't go through analyzeBlock (which does schema-based
		// validation). analyzeLetBlock handles duplicate key detection
		// and reference validation for let values instead.
		if block.TokenStart.Type == token.LET {
			blockDiags = analyzeLetBlock(block, symbols)
		} else {
			blockDiags = analyzeBlock(block, symbols)
			// Apply block-level @suppress annotations.
			codes, all := suppressedCodes(block.Annotations)
			blockDiags = filterSuppressed(blockDiags, codes, all)
		}
		// Tag diagnostics with the source file for multi-file compilation.
		for i := range blockDiags {
			blockDiags[i].File = block.SourceFile
		}
		diags = append(diags, blockDiags...)
	}

	return AnalyzedProgram{
		Ast:         program,
		SymbolTable: symbols,
		Diagnostics: diags,
	}
}

// buildSymbolTable walks all block statements and registers each block
// name with its block reference type. Reports duplicate block names.
func buildSymbolTable(program *ast.Program) (*types.SymbolTable, []diagnostic.Diagnostic) {
	st := types.NewSymbolTable()
	var diags []diagnostic.Diagnostic

	// Seed with built-in schema names (str, int, model, agent, etc.)
	// so they are recognized as valid references in user code.
	// Block types like "model" resolve to their own kind; primitives
	// like "str" resolve to BlockRef(schema).
	for _, name := range types.BuiltinSchemaNames() {
		tokType := token.LookupIdent(name)
		kind, ok := token.TokenTypeToBlockKind(tokType)
		if !ok {
			kind = token.BlockSchema
		}
		st.Define(name, types.NewBlockRefType(kind), token.Token{})
	}

	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}

		// Let blocks register each key as a global symbol.
		// Multiple let blocks are merged; duplicate keys are an error.
		if block.TokenStart.Type == token.LET {
			for _, assign := range block.Assignments {
				if _, exists := st.Lookup(assign.Name); exists {
					diags = append(diags, diagnostic.Diagnostic{
						Severity: diagnostic.Error,
						Code:     diagnostic.CodeDuplicateBlock,
						Position: diagnostic.Position{
							Line:   assign.TokenStart.Line,
							Column: assign.TokenStart.Column,
						},
						Message: fmt.Sprintf("let variable %q conflicts with an existing name", assign.Name),
						Source:  "analyzer",
						File:    block.SourceFile,
					})
					continue
				}
				typ := types.ExprType(assign.Value, st)
				st.Define(assign.Name, typ, assign.TokenStart)
			}
			continue
		}

		kind, ok := token.TokenTypeToBlockKind(block.TokenStart.Type)
		if !ok {
			continue
		}

		if _, exists := st.Lookup(block.Name); exists {
			codes, all := suppressedCodes(block.Annotations)
			if !all && !codes[diagnostic.CodeDuplicateBlock] {
				diags = append(diags, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     diagnostic.CodeDuplicateBlock,
					Position: diagnostic.Position{
						Line:   block.NameToken.Line,
						Column: block.NameToken.Column,
					},
					Message: fmt.Sprintf("duplicate block name %q", block.Name),
					Source:  "analyzer",
					File:    block.SourceFile,
				})
			}
		}
		// For input blocks, resolve the declared type so that member
		// access works through the schema (e.g. vpc_data.region) and
		// type checking shows the actual type (e.g. list[str]) instead
		// of just "input".
		typ := types.NewBlockRefType(kind)
		if kind == token.BlockInput {
			if declared, ok := inputDeclaredType(block); ok {
				typ = declared
			}
		}
		st.Define(block.Name, typ, block.NameToken)
	}
	return st, diags
}

// registerUserSchemas processes user-defined schema blocks and registers
// their field schemas using the same SchemaFromBlock as the built-in
// schema loader.
func registerUserSchemas(program *ast.Program) {
	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok || block.TokenStart.Type != token.SCHEMA {
			continue
		}
		schema, err := types.SchemaFromBlock(block)
		if err != nil {
			continue
		}
		types.RegisterSchema(block.Name, schema)
	}
}

// inputDeclaredType resolves the type from an input block's type field.
// Handles all type expressions: simple identifiers (str), parameterized
// types (list[str], map[str]), inline schemas (schema { ... }), and
// union types (str | null). Returns the resolved Type and true if found.
func inputDeclaredType(block *ast.BlockStatement) (types.Type, bool) {
	for _, assign := range block.Assignments {
		if assign.Name == "type" {
			typ := types.ExprType(assign.Value, nil)
			return typ, true
		}
	}
	return types.Type{}, false
}

// analyzeBlock performs all validation checks on a single block statement.
// Checks for duplicate fields, unknown fields, missing required fields,
// undefined references, and type mismatches.
func analyzeBlock(block *ast.BlockStatement, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	kind, ok := token.TokenTypeToBlockKind(block.TokenStart.Type)
	if !ok {
		return nil
	}
	// Use the generic "schema" schema here, not the block's own definition.
	// Schema blocks are type declarations (e.g. `region = str` means "region
	// is of type str"), not regular blocks with typed values. Validating
	// against the block's own schema would incorrectly type-check declaration
	// values as field assignments.
	schema, ok := types.GetBlockSchema(kind)
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
				Code:     diagnostic.CodeDuplicateField,
				Position: diagnostic.Position{
					Line:   assign.Start().Line,
					Column: assign.Start().Column,
				},
				Message: fmt.Sprintf("duplicate field %q in %s %q", assign.Name, kind.String(), block.Name),
				Source:  "analyzer",
			})
		}
		seen[assign.Name] = true
		fieldDiags := validateField(assign, kind, schema, symbols)
		// Apply field-level @suppress annotations.
		fieldCodes, fieldAll := suppressedCodes(assign.Annotations)
		fieldDiags = filterSuppressed(fieldDiags, fieldCodes, fieldAll)
		diags = append(diags, fieldDiags...)
	}

	// Validate expressions: only workflow blocks allow bare expressions,
	// and workflow expressions must only use the -> operator.
	if kind == token.BlockWorkflow {
		for _, expr := range block.Expressions {
			diags = append(diags, validateWorkflowExpr(expr, symbols)...)
		}
	} else {
		for _, expr := range block.Expressions {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeUnexpectedExpr,
				Position: diagnostic.Position{
					Line:   expr.Start().Line,
					Column: expr.Start().Column,
				},
				Message: fmt.Sprintf("unexpected expression in %s block", kind.String()),
				Source:  "analyzer",
			})
		}
	}

	// Check for missing required fields.
	for fieldName, fieldSchema := range schema.Fields {
		if fieldSchema.Required && !seen[fieldName] {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeMissingField,
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
func validateField(assign *ast.Assignment, kind token.BlockKind, schema types.BlockSchema, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	fieldSchema, ok := schema.Fields[assign.Name]
	if !ok {
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     diagnostic.CodeUnknownField,
			Position: diagnostic.Position{
				Line:   assign.Start().Line,
				Column: assign.Start().Column,
			},
			Message: fmt.Sprintf("unknown field %q in %s block", assign.Name, kind.String()),
			Source:  "analyzer",
		}}
	}

	// Skip validation if the value is nil (incomplete parse).
	if assign.Value == nil {
		return nil
	}

	// Check for undefined references in identifiers and member access.
	if diags := checkReferences(assign.Value, symbols); len(diags) > 0 {
		return diags
	}

	exprType := types.ExprType(assign.Value, symbols)
	// Skip type validation when the expression type is unknown.
	if exprType.IsAny() {
		return nil
	}

	expected := fieldSchema.Type
	if types.IsCompatible(exprType, expected) {
		return nil
	}

	end := assign.Value.End()
	return []diagnostic.Diagnostic{{
		Severity: diagnostic.Error,
		Code:     diagnostic.CodeTypeMismatch,
		Position: diagnostic.Position{
			Line:   assign.Value.Start().Line,
			Column: assign.Value.Start().Column,
		},
		EndPosition: diagnostic.Position{
			Line:   end.EndLine,
			Column: end.EndCol + 1,
		},
		Message: fmt.Sprintf("field %q expects type %s, got %s",
			assign.Name, expected.String(), exprType.String()),
		Source: "analyzer",
	}}
}

// checkReferences recursively validates all identifier and member access
// expressions, reporting errors for undefined block references and unknown members.
func checkReferences(expr ast.Expression, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		if e == nil {
			return nil
		}
		if _, found := symbols.Lookup(e.Value); !found {
			return []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeUndefinedRef,
				Position: diagnostic.Position{
					Line:   e.Start().Line,
					Column: e.Start().Column,
				},
				Message: fmt.Sprintf("undefined reference %q", e.Value),
				Source:  "analyzer",
			}}
		}
	case *ast.MemberAccess:
		if e == nil {
			return nil
		}
		if diags := checkReferences(e.Object, symbols); len(diags) > 0 {
			return diags
		}
		// Skip member validation for incomplete member access (empty Member
		// from partial parse, e.g. "gpt4." while typing).
		if e.Member == "" {
			return nil
		}
		objType := types.ExprType(e.Object, symbols)
		if objType.Kind != types.BlockRef {
			return nil
		}
		schema, schemaOk := types.LookupBlockSchema(objType)
		if !schemaOk {
			return nil
		}
		if _, ok := schema.Fields[e.Member]; !ok {
			return []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeUnknownMember,
				Position: diagnostic.Position{
					Line:   e.End().Line,
					Column: e.End().Column,
				},
				Message: fmt.Sprintf("%q has no field %q", objType.BlockKind.String(), e.Member),
				Source:  "analyzer",
			}}
		}
	case *ast.ListLiteral:
		if e == nil {
			return nil
		}
		for _, elem := range e.Elements {
			if diags := checkReferences(elem, symbols); len(diags) > 0 {
				return diags
			}
		}
	case *ast.BinaryExpression:
		if e == nil {
			return nil
		}
		if diags := checkReferences(e.Left, symbols); len(diags) > 0 {
			return diags
		}
		if diags := checkReferences(e.Right, symbols); len(diags) > 0 {
			return diags
		}
	case *ast.Subscription:
		if e == nil {
			return nil
		}
		if diags := checkReferences(e.Object, symbols); len(diags) > 0 {
			return diags
		}
		if e.Index == nil {
			return nil
		}
		if diags := checkReferences(e.Index, symbols); len(diags) > 0 {
			return diags
		}
		objType := types.ExprType(e.Object, symbols)
		if types.IsCompatible(objType, types.Type{Kind: types.List}) {
			idxType := types.ExprType(e.Index, symbols)
			if !idxType.IsAny() && !types.IsCompatible(idxType, types.Int()) {
				return []diagnostic.Diagnostic{{
					Severity: diagnostic.Error,
					Code:     diagnostic.CodeInvalidSubscript,
					Position: diagnostic.Position{
						Line:   e.Index.Start().Line,
						Column: e.Index.Start().Column,
					},
					Message: fmt.Sprintf("list subscript requires an integer index, got %s", idxType.String()),
					Source:  "analyzer",
				}}
			}
		}
	case *ast.CallExpression:
		if e == nil {
			return nil
		}
		if diags := checkReferences(e.Callee, symbols); len(diags) > 0 {
			return diags
		}
		for _, arg := range e.Arguments {
			if diags := checkReferences(arg, symbols); len(diags) > 0 {
				return diags
			}
		}
	case *ast.MapLiteral:
		if e == nil {
			return nil
		}
		for _, entry := range e.Entries {
			if diags := checkReferences(entry.Key, symbols); len(diags) > 0 {
				return diags
			}
			if diags := checkReferences(entry.Value, symbols); len(diags) > 0 {
				return diags
			}
		}
	case *ast.BlockExpression:
		if e == nil {
			return nil
		}
		for _, assign := range e.Assignments {
			if diags := checkReferences(assign.Value, symbols); len(diags) > 0 {
				return diags
			}
		}
	}
	return nil
}

// suppressedCodes extracts the set of diagnostic codes suppressed by
// @suppress annotations. @suppress with no args suppresses all codes.
// @suppress("code1", "code2") suppresses specific codes.
// Returns the set of suppressed codes, and whether all codes are suppressed.
func suppressedCodes(annotations []*ast.Annotation) (codes map[string]bool, all bool) {
	for _, ann := range annotations {
		if ann.Name != "suppress" {
			continue
		}
		if len(ann.Arguments) == 0 {
			return nil, true
		}
		if codes == nil {
			codes = make(map[string]bool)
		}
		for _, arg := range ann.Arguments {
			if strLit, ok := arg.(*ast.StringLiteral); ok {
				codes[strLit.Value] = true
			}
		}
	}
	return codes, false
}

// filterSuppressed removes diagnostics that are suppressed by the given
// annotation set. If suppressAll is true, all diagnostics are removed.
func filterSuppressed(diags []diagnostic.Diagnostic, codes map[string]bool, suppressAll bool) []diagnostic.Diagnostic {
	if suppressAll {
		return nil
	}
	if len(codes) == 0 {
		return diags
	}
	var filtered []diagnostic.Diagnostic
	for _, d := range diags {
		if !codes[d.Code] {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// validateWorkflowExpr checks that a workflow expression only uses the -> operator.
// Any other binary operator (e.g. +, -, *) is reported as an error.
func validateWorkflowExpr(expr ast.Expression, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	if expr == nil {
		return nil
	}
	var diags []diagnostic.Diagnostic
	switch e := expr.(type) {
	case *ast.BinaryExpression:
		if e.Operator.Type != token.ARROW {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeUnexpectedExpr,
				Position: diagnostic.Position{
					Line:   e.Operator.Line,
					Column: e.Operator.Column,
				},
				Message: fmt.Sprintf("unexpected operator %s in workflow block; only '->' is allowed", token.Describe(e.Operator.Type)),
				Source:  "analyzer",
			})
		}
		diags = append(diags, validateWorkflowExpr(e.Left, symbols)...)
		diags = append(diags, validateWorkflowExpr(e.Right, symbols)...)
	case *ast.Identifier:
		diags = append(diags, checkReferences(e, symbols)...)
	default:
		diags = append(diags, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     diagnostic.CodeUnexpectedExpr,
			Position: diagnostic.Position{
				Line:   expr.Start().Line,
				Column: expr.Start().Column,
			},
			Message: "workflow edges must be identifier references connected by '->'",
			Source:  "analyzer",
		})
	}
	return diags
}

// analyzeLetBlock validates references in let block values.
// Let blocks have no schema — any key name is valid.
func analyzeLetBlock(block *ast.BlockStatement, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic

	seen := make(map[string]bool, len(block.Assignments))
	for _, assign := range block.Assignments {
		if seen[assign.Name] {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeDuplicateField,
				Position: diagnostic.Position{
					Line:   assign.Start().Line,
					Column: assign.Start().Column,
				},
				Message: fmt.Sprintf("duplicate variable %q in let block", assign.Name),
				Source:  "analyzer",
			})
		}
		seen[assign.Name] = true
		if refDiags := checkReferences(assign.Value, symbols); len(refDiags) > 0 {
			diags = append(diags, refDiags...)
		}
	}

	return diags
}

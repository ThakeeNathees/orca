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

// AnalyzeResult holds the output of semantic analysis: the symbol table
// built from block definitions and any diagnostics produced.
type AnalyzeResult struct {
	Symbols     *types.SymbolTable
	Diagnostics []diagnostic.Diagnostic
}

// Analyze walks the AST and performs semantic analysis.
// Builds a symbol table from all block definitions, then validates
// each block's fields against its schema. Returns the symbol table
// along with diagnostics so callers (like the LSP) can use it for
// hover, go-to-definition, and other features.
func Analyze(program *ast.Program) AnalyzeResult {
	var diags []diagnostic.Diagnostic

	symbols, dupDiags := buildSymbolTable(program)
	registerUserSchemas(program)
	diags = append(diags, dupDiags...)

	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		blockDiags := analyzeBlock(block, symbols)
		// Apply block-level @suppress annotations.
		codes, all := suppressedCodes(block.Annotations)
		blockDiags = filterSuppressed(blockDiags, codes, all)
		diags = append(diags, blockDiags...)
	}

	return AnalyzeResult{Symbols: symbols, Diagnostics: diags}
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
		kind, ok := types.BlockKindFromName(name)
		if !ok {
			kind = types.BlockSchemaKind
		}
		st.Define(name, types.NewBlockRefType(kind), token.Token{})
	}

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
				})
			}
		}
		// For input blocks, resolve the declared type so that member
		// access works through the schema (e.g. vpc_data.region).
		typ := types.NewBlockRefType(kind)
		if kind == types.BlockInput {
			if declared := inputDeclaredType(block); declared != "" {
				typ = types.Type{Kind: types.BlockRef, BlockType: types.BlockKind(declared)}
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

// inputDeclaredType extracts the schema name from an input block's type field.
// For `input x { type = vpc_data_t }`, returns "vpc_data_t".
// Returns empty string if the type field is missing or not an identifier.
func inputDeclaredType(block *ast.BlockStatement) string {
	for _, assign := range block.Assignments {
		if assign.Name == "type" {
			if ident, ok := assign.Value.(*ast.Identifier); ok {
				return ident.Value
			}
		}
	}
	return ""
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
				Code:     diagnostic.CodeDuplicateField,
				Position: diagnostic.Position{
					Line:   assign.Start().Line,
					Column: assign.Start().Column,
				},
				Message: fmt.Sprintf("duplicate field %q in %s %q", assign.Name, typeName, block.Name),
				Source:  "analyzer",
			})
		}
		seen[assign.Name] = true
		fieldDiags := validateField(block, assign, schema, symbols)
		// Apply field-level @suppress annotations.
		fieldCodes, fieldAll := suppressedCodes(assign.Annotations)
		fieldDiags = filterSuppressed(fieldDiags, fieldCodes, fieldAll)
		diags = append(diags, fieldDiags...)
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
func validateField(block *ast.BlockStatement, assign *ast.Assignment, schema types.BlockSchema, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	fieldSchema, ok := schema.Fields[assign.Name]
	if !ok {
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     diagnostic.CodeUnknownField,
			Position: diagnostic.Position{
				Line:   assign.Start().Line,
				Column: assign.Start().Column,
			},
			Message: fmt.Sprintf("unknown field %q in %s block", assign.Name, token.BlockName(block.TokenStart.Type)),
			Source:  "analyzer",
		}}
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
	if typeMatches(exprType, expected) {
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
	switch e := expr.(type) {
	case *ast.Identifier:
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
		// First check the object is defined.
		if diags := checkReferences(e.Object, symbols); len(diags) > 0 {
			return diags
		}
		// Then check the member exists on the object's type.
		objType := types.ExprType(e.Object, symbols)
		if objType.Kind != types.BlockRef {
			return nil
		}
		schema, ok := types.GetBlockSchema(string(objType.BlockType))
		if !ok {
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
				Message: fmt.Sprintf("%q has no field %q", objType.BlockType, e.Member),
				Source:  "analyzer",
			}}
		}
	case *ast.ListLiteral:
		for _, elem := range e.Elements {
			if diags := checkReferences(elem, symbols); len(diags) > 0 {
				return diags
			}
		}
	case *ast.BinaryExpression:
		if diags := checkReferences(e.Left, symbols); len(diags) > 0 {
			return diags
		}
		if diags := checkReferences(e.Right, symbols); len(diags) > 0 {
			return diags
		}
	case *ast.Subscription:
		if diags := checkReferences(e.Object, symbols); len(diags) > 0 {
			return diags
		}
		if diags := checkReferences(e.Index, symbols); len(diags) > 0 {
			return diags
		}
		// List subscripts must use an integer index.
		objType := types.ExprType(e.Object, symbols)
		if types.IsCompatible(objType, types.Type{Kind: types.List}) {
			idxType := types.ExprType(e.Index, symbols)
			if !idxType.IsAny() && !types.IsCompatible(idxType, types.TypeOf("int")) {
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
		if diags := checkReferences(e.Callee, symbols); len(diags) > 0 {
			return diags
		}
		for _, arg := range e.Arguments {
			if diags := checkReferences(arg, symbols); len(diags) > 0 {
				return diags
			}
		}
	case *ast.MapLiteral:
		for _, entry := range e.Entries {
			if diags := checkReferences(entry.Key, symbols); len(diags) > 0 {
				return diags
			}
			if diags := checkReferences(entry.Value, symbols); len(diags) > 0 {
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

// typeMatches returns true if the expression type is compatible with the
// expected schema type. Handles unions (expr matches if it matches any member)
// and Any (always matches).
func typeMatches(expr, expected types.Type) bool {
	if expected.IsAny() {
		return true
	}
	if expected.Kind == types.Union {
		return expected.Contains(expr)
	}
	// For lists, match if kinds match (ignore element type for now).
	if expected.Kind == types.List && expr.Kind == types.List {
		return true
	}
	// For BlockRef types, compare by name.
	if expected.Kind == types.BlockRef && expr.Kind == types.BlockRef {
		return expr.BlockType == expected.BlockType
	}
	return expr.Kind == expected.Kind
}

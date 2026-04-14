package analyzer

import (
	"fmt"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
)

// analyzeBlock validates a top-level block statement by delegating to
// analyzeBlockBody for the core body validation.
func analyzeBlock(block *ast.BlockStatement, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	diags := analyzeBlockBody(
		&block.BlockBody,
		block.Annotations,
		block.Name,
		block.OpenBrace,
		block.TokenEnd,
		symbols,
	)
	return diags
}

// analyzeBlockBody performs all validation checks on a block body: duplicate
// fields, unknown fields, missing required fields, undefined references, and
// type mismatches. Shared by both top-level BlockStatement and inline
// BlockExpression so that both get identical validation.
func analyzeBlockBody(
	body *ast.BlockBody,
	_ []*ast.Annotation,
	name string,
	openBrace token.Token,
	endToken token.Token,
	symbols *types.SymbolTable,
) []diagnostic.Diagnostic {

	var diags []diagnostic.Diagnostic

	// Get the block blockSchema
	var blockSchema *types.BlockSchema = nil
	if ty, ok := symbols.Lookup(name); ok {
		blockSchema = ty.Block

		// This is a bug actually cause the name should exists in the symbol table.
		// possibly we forgot to load the bootstrap files or someting.
		if blockSchema == nil {
			panic(fmt.Sprintf("block schema for %q not found in symbol table", name))
		}

	} else {
		// Probably a bug cause all block bodies should be in the symbol table.
		start, end := diagnostic.RangeOf(body)
		return []diagnostic.Diagnostic{{
			Severity:    diagnostic.Error,
			Code:        diagnostic.CodeUndefinedRef,
			Position:    start,
			EndPosition: end,
			Message:     fmt.Sprintf("undefined reference %q", name),
			Source:      "analyzer",
		}}
	}

	// Check for duplicate fields in the body.
	fieldSeen := make(map[string]token.Token, len(body.Assignments))
	for _, assign := range body.Assignments {
		if prevTok, exists := fieldSeen[assign.Name]; exists {
			start, end := diagnostic.RangeOf(assign)
			diags = append(diags, diagnostic.Diagnostic{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeDuplicateField,
				Position:    start,
				EndPosition: end,
				Message:     fmt.Sprintf("duplicate field %q (previously defined at line %d, column %d)", assign.Name, prevTok.Line, prevTok.Column),
				Source:      "analyzer",
			})
		} else {
			fieldSeen[assign.Name] = assign.NameToken
		}
	}

	// If the block is an arbitary block that doesnt have a schema defined (but user anyways use it)
	// example the let block, then we cant do any schema validation, and allow all assignment inside.
	//
	// TODO: Here we're skipping the `schema` (we dont validate the fields of schema with anything)
	// However we have to validate all the fields are schemas.
	//
	// ex: schema foo { a = bar b = baz }
	//
	// bar and baz should be schemas `schema bar {}` and `schema baz {}` (ex: schema string {}).
	// Validate assignments: full schema validation when a schema is available,
	// reference-only validation for schema-less blocks (e.g. let, custom kinds).
	// Schema blocks are skipped — their fields are type names, not value expressions.
	if body.Kind != types.BlockKindSchema {
		hasSchema := blockSchema.Schema != nil && blockSchema.Ast.Kind != types.BlockKindSchema
		for _, assign := range body.Assignments {
			fieldCodes, fieldAll := suppressedCodes(assign.Annotations)
			if hasSchema {
				fieldDiags := validateField(assign, body.Kind, *blockSchema.Schema, symbols)
				diags = append(diags, filterSuppressed(fieldDiags, fieldCodes, fieldAll)...)
			} else if assign.Value != nil {
				refDiags := analyzeExpression(assign.Value, symbols)
				diags = append(diags, filterSuppressed(refDiags, fieldCodes, fieldAll)...)
			}
		}
	}

	// Check if the block support expressions (other than assignments).
	if onlyAssignments := types.HasAnnotation(blockSchema.Annotations, types.AnnotationOnlyAssignments); onlyAssignments {
		for _, expr := range body.Expressions {
			// TODO: once assignments become expressions, we need to check that here.
			start, end := diagnostic.RangeOf(expr)
			diags = append(diags, diagnostic.Diagnostic{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeUnexpectedExpr,
				Position:    start,
				EndPosition: end,
				Message:     fmt.Sprintf("unexpected expression in %s block", body.Kind),
				Source:      "analyzer",
			})
		}
	}

	// Validate expressions: only workflow blocks allow bare expressions.
	if body.Kind == types.BlockKindWorkflow {
		for _, expr := range body.Expressions {
			diags = append(diags, validateWorkflowExpr(expr, symbols)...)
		}

		// TODO: This used to be a function we removed it some time ago while doing some refactoring
		// search in commit history to bring this back.
		//
		// validateWorkflowEntryNodes checks the cardinality rules for workflow entry nodes:
		//   - 0 triggers + 2+ entry nodes → error (ambiguous start)
		//   - 1+ triggers + dangling untriggered entry nodes → warning (unreachable)
		// diags = append(diags, validateWorkflowEntryNodes(name, body.Expressions, symbols)...)
	}

	// Check for missing required fields.
	var seen = make(map[string]bool, len(body.Assignments))
	for _, assign := range body.Assignments {
		seen[assign.Name] = true
	}

	// Report missing required fields if the block has a schema defined.
	if blockSchema.Schema != nil {
		for fieldName, fieldSchema := range blockSchema.Schema.Fields {
			if fieldSchema.Required && !seen[fieldName] {
				diags = append(diags, diagnostic.Diagnostic{
					Severity:    diagnostic.Error,
					Code:        diagnostic.CodeMissingField,
					Position:    diagnostic.PositionOf(openBrace),
					EndPosition: diagnostic.EndPositionOf(endToken),
					Message:     fmt.Sprintf("block %q is missing required field %q", name, fieldName),
					Source:      "analyzer",
				})
			}
		}
	}

	return diags
}

// validateField checks a single field assignment against the block's schema.
// Reports unknown fields, undefined identifier references, and type mismatches.
func validateField(assign *ast.Assignment, kind string, schema types.BlockSchema, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	fieldSchema, ok := schema.Fields[assign.Name]
	if !ok {
		start, end := diagnostic.RangeOf(assign)
		return []diagnostic.Diagnostic{{
			Severity:    diagnostic.Error,
			Code:        diagnostic.CodeUnknownField,
			Position:    start,
			EndPosition: end,
			Message:     fmt.Sprintf("unknown field %q in %s block", assign.Name, kind),
			Source:      "analyzer",
		}}
	}

	// Skip validation if the value is nil (incomplete parse).
	if assign.Value == nil {
		return nil
	}

	// Check for undefined references in identifiers and member access.
	if diags := analyzeExpression(assign.Value, symbols); len(diags) > 0 {
		return diags
	}

	exprType := types.TypeOf(assign.Value, symbols)
	// Skip type validation when the expression type is unknown.
	if exprType.IsAny() {
		return nil
	}

	expected := fieldSchema.Type
	if !types.IsCompatible(exprType, expected) {
		start, end := diagnostic.RangeOf(assign.Value)
		return []diagnostic.Diagnostic{{
			Severity:    diagnostic.Error,
			Code:        diagnostic.CodeTypeMismatch,
			Position:    start,
			EndPosition: end,
			Message: fmt.Sprintf("field %q expects type %s, got %s",
				assign.Name, expected.String(), exprType.String()),
			Source: "analyzer",
		}}
	}

	return nil
}

// analyzeExpression recursively validates all identifier and member access
// expressions, reporting errors for undefined block references and unknown members.
func analyzeExpression(expr ast.Expression, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		if e == nil {
			return nil
		}
		if _, found := symbols.Lookup(e.Value); !found {
			start, end := diagnostic.RangeOf(e)
			return []diagnostic.Diagnostic{{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeUndefinedRef,
				Position:    start,
				EndPosition: end,
				Message:     fmt.Sprintf("undefined reference %q", e.Value),
				Source:      "analyzer",
			}}
		}
	case *ast.MemberAccess:
		if e == nil {
			return nil
		}
		if diags := analyzeExpression(e.Object, symbols); len(diags) > 0 {
			return diags
		}
		// Skip member validation for incomplete member access (empty Member
		// from partial parse, e.g. "gpt4." while typing).
		if e.Member == "" {
			return nil
		}
		objType := types.TypeOf(e.Object, symbols)
		if objType.Kind != types.BlockRef {
			return nil
		}
		if objType.Block == nil {
			return nil
		}
		if _, ok := objType.Block.Fields[e.Member]; !ok {
			return []diagnostic.Diagnostic{{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeUnknownMember,
				Position:    diagnostic.PositionOf(e.End()),
				EndPosition: diagnostic.EndPositionOf(e.End()),
				Message:     fmt.Sprintf("%q has no field %q", objType.BlockName, e.Member),
				Source:      "analyzer",
			}}
		}
	case *ast.ListLiteral:
		if e == nil {
			return nil
		}
		for _, elem := range e.Elements {
			if diags := analyzeExpression(elem, symbols); len(diags) > 0 {
				return diags
			}
		}
	case *ast.BinaryExpression:
		if e == nil {
			return nil
		}
		if diags := analyzeExpression(e.Left, symbols); len(diags) > 0 {
			return diags
		}
		if diags := analyzeExpression(e.Right, symbols); len(diags) > 0 {
			return diags
		}
	case *ast.Subscription:
		if e == nil {
			return nil
		}
		if diags := analyzeExpression(e.Object, symbols); len(diags) > 0 {
			return diags
		}
		for _, idx := range e.Indices {
			if diags := analyzeExpression(idx, symbols); len(diags) > 0 {
				return diags
			}
		}
		objType := types.TypeOf(e.Object, symbols)
		if types.IsCompatible(objType, types.Type{Kind: types.List}) && len(e.Indices) > 0 {
			if len(e.Indices) > 1 {
				start, end := diagnostic.RangeOf(e.Indices[1])
				return []diagnostic.Diagnostic{{
					Severity:    diagnostic.Error,
					Code:        diagnostic.CodeInvalidSubscript,
					Position:    start,
					EndPosition: end,
					Message:     fmt.Sprintf("list subscript expects a single index, got %d", len(e.Indices)),
					Source:      "analyzer",
				}}
			}
			idxType := types.TypeOf(e.Indices[0], symbols)

			// TODO: Const fold and validate out of bounds errors.

			if !idxType.IsAny() && !types.IsCompatible(idxType, types.IdentType(0, types.BlockKindNumber, symbols)) {
				start, end := diagnostic.RangeOf(e.Indices[0])
				return []diagnostic.Diagnostic{{
					Severity:    diagnostic.Error,
					Code:        diagnostic.CodeInvalidSubscript,
					Position:    start,
					EndPosition: end,
					Message:     fmt.Sprintf("list subscript requires an integer index, got %s", idxType.String()),
					Source:      "analyzer",
				}}
			}
		}
		if types.IsCompatible(objType, types.Type{Kind: types.Map}) && len(e.Indices) > 1 {
			start, end := diagnostic.RangeOf(e.Indices[1])
			return []diagnostic.Diagnostic{{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeInvalidSubscript,
				Position:    start,
				EndPosition: end,
				Message:     fmt.Sprintf("map subscript expects a single index, got %d", len(e.Indices)),
				Source:      "analyzer",
			}}
		}
	case *ast.CallExpression:
		if e == nil {
			return nil
		}
		if diags := analyzeExpression(e.Callee, symbols); len(diags) > 0 {
			return diags
		}
		for _, arg := range e.Arguments {
			if diags := analyzeExpression(arg, symbols); len(diags) > 0 {
				return diags
			}
		}

		// TODO: Check if the e.Callee is callable and validate argument match parameters.

	case *ast.MapLiteral:
		if e == nil {
			return nil
		}
		for _, entry := range e.Entries {
			if diags := analyzeExpression(entry.Key, symbols); len(diags) > 0 {
				return diags
			}
			if diags := analyzeExpression(entry.Value, symbols); len(diags) > 0 {
				return diags
			}
		}
	case *ast.TernaryExpression:
		if e == nil {
			return nil
		}
		if diags := analyzeExpression(e.Condition, symbols); len(diags) > 0 {
			return diags
		}
		if diags := analyzeExpression(e.TrueExpr, symbols); len(diags) > 0 {
			return diags
		}
		if diags := analyzeExpression(e.FalseExpr, symbols); len(diags) > 0 {
			return diags
		}
	case *ast.Lambda:
		if e == nil {
			return nil
		}
		// Check param type expressions and return type against current scope
		// (before pushing params).
		for _, p := range e.Params {
			if diags := analyzeExpression(p.TypeExpr, symbols); len(diags) > 0 {
				return diags
			}
		}
		if e.ReturnType != nil {
			if diags := analyzeExpression(e.ReturnType, symbols); len(diags) > 0 {
				return diags
			}
		}
		// Push a child scope for lambda parameters.
		symbols.PushScope()
		for _, p := range e.Params {
			// Use depth 0 to get the direct type (e.g. "number" → Type{BlockRef, "number", <schema number {}>})
			// rather than depth 1 which walks up to the meta-schema.
			paramType := types.EvalType(p.TypeExpr, symbols)
			// Create a synthetic block instance for the param so IdentType's
			// depth chain resolves correctly. E.g. param `n number` gets a block
			// with Ast.Kind="number", mirroring how `model gpt4 {}` works.
			paramSchema := types.NewLambdaParamSchema(p.Name.Value, paramType)
			typ := types.NewBlockRefType(p.Name.Value, &paramSchema)
			symbols.Define(p.Name.Value, typ, p.Name.Start())
		}
		// Check body against scope with params visible.
		if diags := analyzeExpression(e.Body, symbols); len(diags) > 0 {
			symbols.PopScope()
			return diags
		}
		// Validate body type matches declared return type.
		if e.ReturnType != nil {
			expected := types.EvalType(e.ReturnType, symbols)
			got := types.TypeOf(e.Body, symbols)
			if !types.IsCompatible(got, expected) {
				symbols.PopScope()
				start, end := diagnostic.RangeOf(e.Body)
				return []diagnostic.Diagnostic{{
					Severity:    diagnostic.Error,
					Code:        diagnostic.CodeTypeMismatch,
					Position:    start,
					EndPosition: end,
					Message:     fmt.Sprintf("lambda body type %s does not match declared return type %s", got.String(), expected.String()),
					Source:      "analyzer",
				}}
			}
		}
		symbols.PopScope()
	case *ast.BlockExpression:
		if e == nil {
			return nil
		}
		diags := analyzeBlockBody(&e.BlockBody, nil, e.Name, e.TokenStart, e.TokenEnd, symbols)
		if len(diags) > 0 {
			return diags
		}
	}
	return nil
}

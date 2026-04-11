// Package analyzer performs semantic analysis on the AST produced by the parser.
// It resolves references between blocks (e.g., verifying that an agent's model
// refers to a defined model block), checks for type mismatches, undefined
// identifiers, and other errors that can't be caught by syntax alone.
package analyzer

import (
	"fmt"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/graph"
	"github.com/thakee/orca/compiler/helper"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
	"github.com/thakee/orca/compiler/workflow"
)

// AnalyzedProgram holds the output of semantic analysis: the symbol table
// built from block definitions and any diagnostics produced.
type AnalyzedProgram struct {
	Ast         *ast.Program
	SymbolTable *types.SymbolTable
	Diagnostics []diagnostic.Diagnostic

	// BlockOrder is the topologically sorted list of user-defined block names.
	// Blocks with no dependencies come first; dependents come after their
	// dependencies. Codegen uses this to emit blocks in valid definition order.
	BlockOrder []string
}

// Analyze walks the AST and performs semantic analysis.
// Builds a symbol table from all block definitions, then validates
// each block's fields against its schema. Returns the symbol table
// along with diagnostics so callers (like the LSP) can use it for
// hover, go-to-definition, and other features.
func Analyze(program *ast.Program) AnalyzedProgram {

	// Bootstrap the schema file.
	bootstrapResult := types.Bootstrap(types.BootstrapSource)

	ap := AnalyzedProgram{
		Ast:         program,
		SymbolTable: bootstrapResult.Symtab,
		Diagnostics: []diagnostic.Diagnostic{},
	}

	buildSymbolTable(&ap)
	resolveBlockSchemaReferences(&ap)
	buildBlockDependencyGraph(&ap)

	for _, stmt := range program.Statements {
		// We dont have any other statement than BlockStatement, maybe
		// we can just remove Statement and use BlockStatement directly.
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}

		// Analyze and get the non-suppressed diagnostics for the block.
		var blockDiags []diagnostic.Diagnostic
		blockDiags = analyzeBlock(block, ap.SymbolTable)
		codes, all := suppressedCodes(block.Annotations)
		blockDiags = filterSuppressed(blockDiags, codes, all)

		// Add the block diagnostics to the program diagnostics.
		ap.Diagnostics = append(ap.Diagnostics, blockDiags...)
	}

	return ap
}

// buildSymbolTable walks all block statements and registers each block
// name with its block reference type. Reports duplicate block names.
func buildSymbolTable(ap *AnalyzedProgram) {

	for _, stmt := range ap.Ast.Statements {

		// NOTE: Block is and will always be the only statement supported by Orca
		// Probably we dont need this generic statement base class.
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}

		// TODO: Split the bellow logic into smaller function for readability.
		if _, exists := ap.SymbolTable.Lookup(block.Name); exists {
			codes, all := suppressedCodes(block.Annotations)
			if !all && !codes[diagnostic.CodeDuplicateBlock] {
				ap.Diagnostics = append(ap.Diagnostics, diagnostic.Diagnostic{
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

		schema := types.NewBlockSchema(
			block.Annotations,
			block.Name,
			&block.BlockBody,
			ap.SymbolTable)

		// Define the block in the symbol table.
		typ := types.NewBlockRefType(block.Name, &schema)
		ap.SymbolTable.Define(block.Name, typ, block.NameToken)
	}
}

// FIXME: This might cause a stack overflow if the schema is recursive.
// Add a depth parameter to the function.
func resolveFieldSchemaReferences(bs *types.BlockSchema, st *types.SymbolTable) {
	for _, fieldSchema := range bs.Fields {
		resolveTypeBlockReference(&fieldSchema.Type, st)
	}
}

func resolveTypeBlockReference(typ *types.Type, st *types.SymbolTable) {
	if typ.Kind != types.BlockRef {
		return
	}

	if typ.Block == nil {
		if ref, ok := st.Lookup(typ.BlockName); ok {
			typ.Block = ref.Block
		}
	}

	if typ.Block != nil {
		resolveFieldSchemaReferences(typ.Block, st)
	}

	if typ.Block != nil && typ.Block.Schema == nil {
		if schemaRef, ok := st.Lookup(typ.Block.Ast.Kind); ok {
			typ.Block.Schema = schemaRef.Block
		}
	}

	if typ.Block != nil && typ.Block.Schema != nil {
		resolveFieldSchemaReferences(typ.Block.Schema, st)
	}
}

func resolveBlockSchemaReferences(ap *AnalyzedProgram) {

	// Bootstrap the schema's schema.
	//
	//   +---------------------------------------+
	//   | schema's schema is the schema itself. |
	//   +---------------------------------------+
	//
	if schemaSchema, ok := ap.SymbolTable.Lookup(types.BlockKindSchema); ok {
		schemaSchema.Block.Schema = schemaSchema.Block
	}

	for _, symbol := range ap.SymbolTable.GetSymbols() {
		resolveTypeBlockReference(&symbol.Type, ap.SymbolTable)
	}
}

// buildBlockDependencyGraph constructs a directed graph of block-to-block
// dependencies by walking each block's expressions for references to other
// user-defined blocks. The graph is topologically sorted to produce a valid
// emission order for codegen. Cycles are reported as diagnostics.
func buildBlockDependencyGraph(ap *AnalyzedProgram) {
	g := graph.New[string]()

	// Collect user-defined block names (everything not from bootstrap).
	userBlocks := make(map[string]*ast.BlockStatement)
	for _, stmt := range ap.Ast.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		userBlocks[block.Name] = block
		g.AddNode(block.Name)
	}

	// Extract dependencies: for each block, walk its body and find
	// references to other user-defined blocks.
	for _, stmt := range ap.Ast.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		deps := make(map[string]bool)
		for _, assign := range block.BlockBody.Assignments {
			if assign.Value != nil {
				collectBlockDeps(assign.Value, userBlocks, deps)
			}
		}
		for _, expr := range block.BlockBody.Expressions {
			collectBlockDeps(expr, userBlocks, deps)
		}
		for dep := range deps {
			if dep != block.Name { // skip self-references (handled by other checks)
				g.AddEdge(dep, block.Name)
			}
		}
	}

	// Topological sort — reverse because edges point from dependent → dependency,
	// so dependencies must be emitted first.
	sorted, err := g.TopologicalSort()
	if err != nil {
		// Report cycle diagnostic on each block involved.
		// Since we can't easily pinpoint exactly which blocks form the cycle,
		// report on all blocks that weren't emitted by the sort.
		ap.Diagnostics = append(ap.Diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     diagnostic.CodeCyclicDependency,
			Position: diagnostic.Position{Line: 1, Column: 1},
			Message:  fmt.Sprintf("block dependency cycle detected: %s", err),
			Source:   "analyzer",
		})
		// Fall back to source order when there's a cycle.
		ap.BlockOrder = g.Nodes()
		return
	}

	ap.BlockOrder = sorted
}

// collectBlockDeps recursively walks an expression and collects the names of
// any user-defined blocks it references.
func collectBlockDeps(expr ast.Expression, userBlocks map[string]*ast.BlockStatement, deps map[string]bool) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		if _, ok := userBlocks[e.Value]; ok {
			deps[e.Value] = true
		}
	case *ast.MemberAccess:
		// The dependency is on the root object, not the member.
		collectBlockDeps(e.Object, userBlocks, deps)
	case *ast.BinaryExpression:
		collectBlockDeps(e.Left, userBlocks, deps)
		collectBlockDeps(e.Right, userBlocks, deps)
	case *ast.ListLiteral:
		for _, elem := range e.Elements {
			collectBlockDeps(elem, userBlocks, deps)
		}
	case *ast.MapLiteral:
		for _, entry := range e.Entries {
			collectBlockDeps(entry.Key, userBlocks, deps)
			collectBlockDeps(entry.Value, userBlocks, deps)
		}
	case *ast.CallExpression:
		collectBlockDeps(e.Callee, userBlocks, deps)
		for _, arg := range e.Arguments {
			collectBlockDeps(arg, userBlocks, deps)
		}
	case *ast.Subscription:
		collectBlockDeps(e.Object, userBlocks, deps)
		for _, idx := range e.Indices {
			collectBlockDeps(idx, userBlocks, deps)
		}
	case *ast.TernaryExpression:
		collectBlockDeps(e.Condition, userBlocks, deps)
		collectBlockDeps(e.TrueExpr, userBlocks, deps)
		collectBlockDeps(e.FalseExpr, userBlocks, deps)
	case *ast.Lambda:
		// Lambda params shadow outer names, but the body may still reference
		// outer blocks. We don't exclude param names from deps because
		// param names are not block names.
		for _, p := range e.Params {
			collectBlockDeps(p.TypeExpr, userBlocks, deps)
		}
		if e.ReturnType != nil {
			collectBlockDeps(e.ReturnType, userBlocks, deps)
		}
		collectBlockDeps(e.Body, userBlocks, deps)
	case *ast.BlockExpression:
		for _, assign := range e.BlockBody.Assignments {
			if assign.Value != nil {
				collectBlockDeps(assign.Value, userBlocks, deps)
			}
		}
		for _, subExpr := range e.BlockBody.Expressions {
			collectBlockDeps(subExpr, userBlocks, deps)
		}
	}
}

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
	// Tag diagnostics with the source file for multi-file compilation.
	for i := range diags {
		diags[i].File = block.SourceFile
	}
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
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     diagnostic.CodeUndefinedRef,
			Position: diagnostic.Position{
				Line:   body.Start().Line,
				Column: body.Start().Column,
			},
			Message: fmt.Sprintf("undefined reference %q", name),
			Source:  "analyzer",
		}}
	}

	// Check for duplicate fields in the body.
	fieldSeen := make(map[string]token.Token, len(body.Assignments))
	for _, assign := range body.Assignments {
		if prevTok, exists := fieldSeen[assign.Name]; exists {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeDuplicateField,
				Position: diagnostic.Position{
					Line:   assign.Start().Line,
					Column: assign.Start().Column,
				},
				EndPosition: diagnostic.Position{
					Line:   assign.End().Line,
					Column: assign.End().Column,
				},
				Message: fmt.Sprintf("duplicate field %q (previously defined at line %d, column %d)", assign.Name, prevTok.Line, prevTok.Column),
				Source:  "analyzer",
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
				fieldDiags := validateField(assign, name, body.Kind, *blockSchema.Schema, symbols)
				diags = append(diags, filterSuppressed(fieldDiags, fieldCodes, fieldAll)...)
			} else if assign.Value != nil {
				refDiags := checkReferences(assign.Value, symbols)
				diags = append(diags, filterSuppressed(refDiags, fieldCodes, fieldAll)...)
			}
		}
	}

	// Check if the block support expressions (other than assignments).
	if onlyAssignments := helper.HasAnnotation(blockSchema.Annotations, AnnotationOnlyAssignments); onlyAssignments {
		for _, expr := range body.Expressions {
			// TODO: once assignments become expressions, we need to check that here.
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeUnexpectedExpr,
				Position: diagnostic.Position{
					Line:   expr.Start().Line,
					Column: expr.Start().Column,
				},
				Message: fmt.Sprintf("unexpected expression in %s block", body.Kind),
				Source:  "analyzer",
			})
		}
	}

	// Validate expressions: only workflow blocks allow bare expressions.
	if body.Kind == BlockKindWorkflow {
		for _, expr := range body.Expressions {
			diags = append(diags, validateWorkflowExpr(expr, symbols)...)
		}
		diags = append(diags, validateTriggerPositions(body.Expressions, symbols)...)
		diags = append(diags, validateWorkflowEntryNodes(name, body.Expressions, symbols)...)
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
					Severity: diagnostic.Error,
					Code:     diagnostic.CodeMissingField,
					Position: diagnostic.Position{
						Line:   openBrace.Line,
						Column: openBrace.Column,
					},
					EndPosition: diagnostic.Position{
						Line:   endToken.Line,
						Column: endToken.Column + 1,
					},
					Message: fmt.Sprintf("block %q is missing required field %q", name, fieldName),
					Source:  "analyzer",
				})
			}
		}
	}

	return diags
}

// validateField checks a single field assignment against the block's schema.
// Reports unknown fields, undefined identifier references, and type mismatches.
func validateField(assign *ast.Assignment, blockName string, kind string, schema types.BlockSchema, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	fieldSchema, ok := schema.Fields[assign.Name]
	if !ok {
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     diagnostic.CodeUnknownField,
			Position: diagnostic.Position{
				Line:   assign.Start().Line,
				Column: assign.Start().Column,
			},
			Message: fmt.Sprintf("unknown field %q in %s block", assign.Name, kind),
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

	exprType := types.SchemaTypeFromExpr(assign.Value, symbols)
	// Skip type validation when the expression type is unknown.
	if exprType.IsAny() {
		return nil
	}

	expected := fieldSchema.Type
	if !types.IsCompatible(exprType, expected) {
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

	return nil
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
		objType := types.SchemaTypeFromExpr(e.Object, symbols)
		if objType.Kind != types.BlockRef {
			return nil
		}
		if objType.Block == nil {
			return nil
		}
		if _, ok := objType.Block.Fields[e.Member]; !ok {
			return []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeUnknownMember,
				Position: diagnostic.Position{
					Line:   e.End().Line,
					Column: e.End().Column,
				},
				Message: fmt.Sprintf("%q has no field %q", objType.BlockName, e.Member),
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
		for _, idx := range e.Indices {
			if diags := checkReferences(idx, symbols); len(diags) > 0 {
				return diags
			}
		}
		objType := types.SchemaTypeFromExpr(e.Object, symbols)
		if types.IsCompatible(objType, types.Type{Kind: types.List}) && len(e.Indices) > 0 {
			if len(e.Indices) > 1 {
				return []diagnostic.Diagnostic{{
					Severity: diagnostic.Error,
					Code:     diagnostic.CodeInvalidSubscript,
					Position: diagnostic.Position{
						Line:   e.Indices[1].Start().Line,
						Column: e.Indices[1].Start().Column,
					},
					Message: fmt.Sprintf("list subscript expects a single index, got %d", len(e.Indices)),
					Source:  "analyzer",
				}}
			}
			idxType := types.SchemaTypeFromExpr(e.Indices[0], symbols)

			// TODO: Const fold and validate out of bounds errors.

			if !idxType.IsAny() && !types.IsCompatible(idxType, types.IdentType(0, BlockKindNumber, symbols)) {
				return []diagnostic.Diagnostic{{
					Severity: diagnostic.Error,
					Code:     diagnostic.CodeInvalidSubscript,
					Position: diagnostic.Position{
						Line:   e.Indices[0].Start().Line,
						Column: e.Indices[0].Start().Column,
					},
					Message: fmt.Sprintf("list subscript requires an integer index, got %s", idxType.String()),
					Source:  "analyzer",
				}}
			}
		}
		if types.IsCompatible(objType, types.Type{Kind: types.Map}) && len(e.Indices) > 1 {
			return []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeInvalidSubscript,
				Position: diagnostic.Position{
					Line:   e.Indices[1].Start().Line,
					Column: e.Indices[1].Start().Column,
				},
				Message: fmt.Sprintf("map subscript expects a single index, got %d", len(e.Indices)),
				Source:  "analyzer",
			}}
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
	case *ast.TernaryExpression:
		if e == nil {
			return nil
		}
		if diags := checkReferences(e.Condition, symbols); len(diags) > 0 {
			return diags
		}
		if diags := checkReferences(e.TrueExpr, symbols); len(diags) > 0 {
			return diags
		}
		if diags := checkReferences(e.FalseExpr, symbols); len(diags) > 0 {
			return diags
		}
	case *ast.Lambda:
		if e == nil {
			return nil
		}
		// Check param type expressions and return type against current scope
		// (before pushing params).
		for _, p := range e.Params {
			if diags := checkReferences(p.TypeExpr, symbols); len(diags) > 0 {
				return diags
			}
		}
		if e.ReturnType != nil {
			if diags := checkReferences(e.ReturnType, symbols); len(diags) > 0 {
				return diags
			}
		}
		// Push a child scope for lambda parameters.
		symbols.PushScope()
		for _, p := range e.Params {
			// Use depth 0 to get the direct type (e.g. "number" → Type{BlockRef, "number", <schema number {}>})
			// rather than depth 1 which walks up to the meta-schema.
			paramType := types.ExprTypeFromExpr(p.TypeExpr, symbols)
			// Create a synthetic block instance for the param so IdentType's
			// depth chain resolves correctly. E.g. param `n number` gets a block
			// with Ast.Kind="number", mirroring how `model gpt4 {}` works.
			paramSchema := types.NewLambdaParamSchema(p.Name.Value, paramType)
			typ := types.NewBlockRefType(p.Name.Value, &paramSchema)
			symbols.Define(p.Name.Value, typ, p.Name.Start())
		}
		// Check body against scope with params visible.
		if diags := checkReferences(e.Body, symbols); len(diags) > 0 {
			symbols.PopScope()
			return diags
		}
		// Validate body type matches declared return type.
		if e.ReturnType != nil {
			expected := types.ExprTypeFromExpr(e.ReturnType, symbols)
			got := types.SchemaTypeFromExpr(e.Body, symbols)
			if !types.IsCompatible(got, expected) {
				symbols.PopScope()
				return []diagnostic.Diagnostic{{
					Severity: diagnostic.Error,
					Code:     diagnostic.CodeTypeMismatch,
					Position: diagnostic.Position{
						Line:   e.Body.Start().Line,
						Column: e.Body.Start().Column,
					},
					EndPosition: diagnostic.Position{
						Line:   e.Body.End().Line,
						Column: e.Body.End().Column + len(e.Body.End().Literal),
					},
					Message: fmt.Sprintf("lambda body type %s does not match declared return type %s", got.String(), expected.String()),
					Source:  "analyzer",
				}}
			}
		}
		symbols.PopScope()
	case *ast.BlockExpression:
		if e == nil {
			return nil
		}
		diags := analyzeBlockBody(&e.BlockBody, nil, e.BlockNameAnon, e.TokenStart, e.TokenEnd, symbols)
		if len(diags) > 0 {
			return diags
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

// validateWorkflowExpr checks that a workflow expression only uses the -> operator
// and that each graph endpoint resolves to a workflow-capable block reference
// (agent, tool, cron, webhook, branch) via the type system.
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

		// Branch blocks cannot be on the left side of -> (no outgoing edges allowed).
		// In a chain like A -> branch{...} -> C, the parse tree is ((A -> branch{...}) -> C),
		// so we check the rightmost leaf of the left subtree.
		if isBranchExpr(rightmostLeaf(e.Left), symbols) {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeBranchOutgoing,
				Position: diagnostic.Position{
					Line:   e.Operator.Line,
					Column: e.Operator.Column,
				},
				Message: "branch block cannot have outgoing edges; define connections in the route table instead",
				Source:  "analyzer",
			})
		}

		diags = append(diags, validateWorkflowExpr(e.Left, symbols)...)
		diags = append(diags, validateWorkflowExpr(e.Right, symbols)...)
	default:
		diags = append(diags, validateWorkflowLeafExpr(expr, symbols)...)
	}
	return diags
}

// rightmostLeaf returns the rightmost non-arrow leaf of an expression tree.
// For A -> B -> C (parsed as (A -> B) -> C), rightmostLeaf returns C.
// For a non-binary expression, returns the expression itself.
func rightmostLeaf(expr ast.Expression) ast.Expression {
	if bin, ok := expr.(*ast.BinaryExpression); ok && bin.Operator.Type == token.ARROW {
		return rightmostLeaf(bin.Right)
	}
	return expr
}

// isBranchExpr returns true if the expression resolves to a branch block type.
// Works for both inline branch { ... } expressions and named branch references.
func isBranchExpr(expr ast.Expression, symbols *types.SymbolTable) bool {
	// Fast path: inline block expression with Kind "branch".
	if be, ok := expr.(*ast.BlockExpression); ok {
		return be.Kind == workflow.BlockKindBranch
	}
	// Named branch reference: resolve the type and check the block kind.
	typ := types.SchemaTypeFromExpr(expr, symbols)
	return typ.Kind == types.BlockRef && typ.BlockName == workflow.BlockKindBranch
}

// validateWorkflowLeafExpr checks a single workflow node position (not an arrow).
// It resolves the expression's type and requires a BlockRef whose kind passes
// IsWorkflowNode(); other types are not valid graph nodes.
func validateWorkflowLeafExpr(expr ast.Expression, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	if refDiags := checkReferences(expr, symbols); len(refDiags) > 0 {
		return refDiags
	}

	// Workflow leaf expression must be a block reference.
	typ := types.SchemaTypeFromExpr(expr, symbols)
	if typ.Kind != types.BlockRef {
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     diagnostic.CodeUnexpectedExpr,
			Position: diagnostic.Position{
				Line:   expr.Start().Line,
				Column: expr.Start().Column,
			},
			Message: "unexpected expression in workflow block",
			Source:  "analyzer",
		}}
	}

	// Ensure the block is a workflow node.
	schema := typ.Block
	if schema == nil || !helper.HasAnnotation(schema.Annotations, AnnotationWorkflowNode) {
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     diagnostic.CodeInvalidWorkNode,
			Position: diagnostic.Position{
				Line:   expr.Start().Line,
				Column: expr.Start().Column,
			},
			Message: fmt.Sprintf("%s block is not a valid workflow node", typ.BlockName),
			Source:  "analyzer",
		}}
	}

	return nil
}

// isTriggerExpr returns true if the expression's resolved type is a trigger block kind.
// Works with any expression type (identifiers, member access, subscriptions, etc.)
// by using the type system to infer the block kind.
func isTriggerExpr(expr ast.Expression, symbols *types.SymbolTable) bool {
	typ := types.SchemaTypeFromExpr(expr, symbols)
	if typ.Kind != types.BlockRef {
		return false
	}
	schema := typ.Block
	if schema == nil {
		return false
	}
	if !helper.HasAnnotation(schema.Annotations, AnnotationTriggerNode) {
		return false
	}
	return true
}

// validateTriggerPositions checks that trigger blocks (cron, webhook) only appear
// as the first node in edge chains and never as the target of another node.
// It also rejects trigger-to-trigger chains (e.g. daily -> hooks_in).
func validateTriggerPositions(exprs []ast.Expression, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic
	for _, expr := range exprs {
		diags = append(diags, checkTriggerChain(expr, symbols)...)
	}
	return diags
}

// checkTriggerChain recursively walks an arrow expression and reports any
// trigger that appears on the right side of ->. The leftmost node in a chain
// is never checked here — it's the only valid position for a trigger.
func checkTriggerChain(expr ast.Expression, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	bin, ok := expr.(*ast.BinaryExpression)
	if !ok || bin.Operator.Type != token.ARROW {
		return nil
	}

	var diags []diagnostic.Diagnostic

	// The parser builds left-associative trees: ((A -> B) -> C).
	diags = append(diags, checkTriggerChain(bin.Left, symbols)...)

	// Right side is always a target — triggers are not allowed.
	if isTriggerExpr(bin.Right, symbols) {
		diags = append(diags, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     diagnostic.CodeTriggerAsTarget,
			Position: diagnostic.Position{Line: bin.Right.Start().Line, Column: bin.Right.Start().Column},
			Message:  "trigger cannot be the target of an edge; triggers must be the first node in a chain",
			Source:   "analyzer",
		})
	}

	return diags
}

// validateWorkflowEntryNodes checks the cardinality rules for workflow entry nodes:
//   - 0 triggers + 2+ entry nodes → error (ambiguous start)
//   - 1+ triggers + dangling untriggered entry nodes → warning (unreachable)
func validateWorkflowEntryNodes(workflowName string, exprs []ast.Expression, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	// Collect all edges and classify trigger nodes.
	var edges []workflow.Edge
	triggers := make(map[string]bool)
	for _, expr := range exprs {
		for _, e := range workflow.EdgesFromExpr(expr) {
			edges = append(edges, e)
		}
		collectTriggerNodes(expr, symbols, triggers)
	}

	if len(edges) == 0 {
		return nil
	}

	// Find entry nodes: processing nodes (non-triggers) with no incoming
	// edges from other processing nodes.
	hasIncoming := make(map[string]bool)
	allNodes := make(map[string]bool)
	for _, e := range edges {
		if !triggers[e.From] {
			allNodes[e.From] = true
		}
		if !triggers[e.To] {
			allNodes[e.To] = true
			if !triggers[e.From] {
				hasIncoming[e.To] = true
			}
		}
	}

	var entryNodes []string
	for node := range allNodes {
		if !hasIncoming[node] {
			entryNodes = append(entryNodes, node)
		}
	}

	// Determine which entry nodes are triggered.
	triggeredEntries := make(map[string]bool)
	for _, e := range edges {
		if triggers[e.From] && !triggers[e.To] {
			triggeredEntries[e.To] = true
		}
	}

	hasTriggers := len(triggers) > 0
	var diags []diagnostic.Diagnostic

	if !hasTriggers && len(entryNodes) > 1 {
		for _, node := range entryNodes {
			pos := findIdentPos(exprs, node)
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeAmbiguousStart,
				Position: pos,
				Message: fmt.Sprintf(
					"workflow %q has multiple entry nodes without triggers; add a trigger or use a single entry node",
					workflowName,
				),
				Source: "analyzer",
			})
		}
	}

	if hasTriggers {
		for _, node := range entryNodes {
			if !triggeredEntries[node] {
				diags = append(diags, diagnostic.Diagnostic{
					Severity: diagnostic.Warning,
					Code:     diagnostic.CodeDanglingEntry,
					Position: findIdentPos(exprs, node),
					Message:  fmt.Sprintf("workflow %q: entry node %q has no trigger and will be unreachable", workflowName, node),
					Source:   "analyzer",
				})
			}
		}
	}

	return diags
}

// collectTriggerNodes walks an arrow expression and records any node that
// resolves to a trigger block kind in the triggers set.
func collectTriggerNodes(expr ast.Expression, symbols *types.SymbolTable, triggers map[string]bool) {
	bin, ok := expr.(*ast.BinaryExpression)
	if !ok || bin.Operator.Type != token.ARROW {
		// Single node expression — check if it's a trigger.
		if isTriggerExpr(expr, symbols) {
			triggers[workflow.ExprToNodeName(expr)] = true
		}
		return
	}

	collectTriggerNodes(bin.Left, symbols, triggers)

	if isTriggerExpr(bin.Right, symbols) {
		triggers[workflow.ExprToNodeName(bin.Right)] = true
	}
}

// findIdentPos searches workflow expressions for the first occurrence of
// an identifier with the given name and returns its source position.
// This is used to attach diagnostics to the exact token in the source,
// e.g. highlighting "analyst" in:
//
//	workflow pipeline {
//	  researcher -> writer  // ← ambiguous-start (could be analyst or researcher)
//	  analyst -> writer     // ← ambiguous-start (could be analyst or researcher)
//	}
func findIdentPos(exprs []ast.Expression, name string) diagnostic.Position {
	for _, expr := range exprs {
		if pos, ok := findIdentInExpr(expr, name); ok {
			return pos
		}
	}
	// Fallback: first expression start.
	if len(exprs) > 0 {
		return diagnostic.Position{Line: exprs[0].Start().Line, Column: exprs[0].Start().Column}
	}
	return diagnostic.Position{}
}

// findIdentInExpr recursively walks an expression tree (including nested
// arrow chains like ((A -> B) -> C)) to find an identifier by name.
// Returns the source position of the first match, or false if not found.
func findIdentInExpr(expr ast.Expression, name string) (diagnostic.Position, bool) {
	switch e := expr.(type) {
	case *ast.Identifier:
		if e.Value == name {
			return diagnostic.Position{Line: e.Start().Line, Column: e.Start().Column}, true
		}
	case *ast.BinaryExpression:
		if pos, ok := findIdentInExpr(e.Left, name); ok {
			return pos, true
		}
		return findIdentInExpr(e.Right, name)
	}
	return diagnostic.Position{}, false
}

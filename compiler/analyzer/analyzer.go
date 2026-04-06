// Package analyzer performs semantic analysis on the AST produced by the parser.
// It resolves references between blocks (e.g., verifying that an agent's model
// refers to a defined model block), checks for type mismatches, undefined
// identifiers, and other errors that can't be caught by syntax alone.
package analyzer

import (
	"fmt"
	"regexp"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/helper"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
	"github.com/thakee/orca/compiler/workflow"
)

// defNameRe matches `def <name>` at the start of a Python function definition
// to extract the function name from an inline invoke raw string.
var defNameRe = regexp.MustCompile(`(?m)^\s*def\s+(\w+)`)

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

	analyzedProgram := AnalyzedProgram{
		Ast:         program,
		SymbolTable: types.NewSymbolTable(),
		Diagnostics: []diagnostic.Diagnostic{},
	}

	st, diags := buildSymbolTable(program)
	analyzedProgram.SymbolTable = st
	analyzedProgram.Diagnostics = append(analyzedProgram.Diagnostics, diags...)

	registerUserSchemas(program)

	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}

		var blockDiags []diagnostic.Diagnostic
		blockDiags = analyzeBlock(block, symbols)

		// Apply block-level @suppress annotations.
		codes, all := suppressedCodes(block.Annotations)
		blockDiags = filterSuppressed(blockDiags, codes, all)

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

	// Define true and false
	st.Define("true", types.Bool(), token.Token{})
	st.Define("false", types.Bool(), token.Token{})

	var diags []diagnostic.Diagnostic

	// TODO: This should be done in the boot loading process itself not here.
	//
	// Seed with built-in schema names (str, int, model, agent, etc.)
	// so they are recognized as valid references in user code.
	// Block types like "model" resolve to their own kind; primitives
	// like "str" resolve to BlockRef(schema).
	for _, builtinNames := range types.BuiltinSchemaNames() {
		st.Define(
			builtinNames,
			types.NewBlockRefType(types.BlockKindSchema, builtinNames),
			token.Token{},
		)
	}

	for _, stmt := range program.Statements {

		// NOTE: Block is and will always be the only statement supported by Orca
		// Probably we dont need this generic statement base class.
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}

		// TODO: Split the bellow logic into smaller function for readability.
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

		schema := types.NewBlockSchema(block.Annotations, block.Name, &block.BlockBody)

		// For input blocks, resolve the declared type so that member
		// access works through the schema (e.g. vpc_data.region) and
		// type checking shows the actual type (e.g. list[str]) instead
		// of just "input".
		typ := types.NewBlockRefType(block.Kind, block.Name)
		if block.Kind == BlockKindInput {
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
		if !ok || block.Kind != types.BlockKindSchema {
			continue
		}
		schema := types.NewBlockSchema(block.Annotations, block.Name, &block.BlockBody)
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

// analyzeBlock validates a top-level block statement by delegating to
// analyzeBlockBody for the core body validation.
func analyzeBlock(block *ast.BlockStatement, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	return analyzeBlockBody(
		&block.BlockBody,
		block.Annotations,
		block.Name,
		block.OpenBrace,
		block.TokenEnd,
		symbols,
	)
}

// analyzeBlockBody performs all validation checks on a block body: duplicate
// fields, unknown fields, missing required fields, undefined references, and
// type mismatches. Shared by both top-level BlockStatement and inline
// BlockExpression so that both get identical validation.
func analyzeBlockBody(
	body *ast.BlockBody,
	annotations []*ast.Annotation,
	name string,
	openBrace token.Token,
	endToken token.Token,
	symbols *types.SymbolTable,
) []diagnostic.Diagnostic {
	kind := body.Kind

	schema, ok := types.GetSchema(kind)
	if !ok {
		// If a schema is not registered (throught the schema <name> {}) this is
		// an arbitary block and we infer the schema from the block body.
		schema = types.NewBlockSchema(annotations, name, body)
		// NOTE that we dont register this schema in the global schema map as:
		//   types.RegisterSchema(kind, schema)
		// Because they dont have a single definition of schema, Example:
		//   let vars { a = 1 b = 2 }
		//   let conf { a = "" b = "" }
		// So the 'let' kind dont have a schema definition, but 'vars' and 'conf'
		// Should be registered into the symbol table for lookups like: vars.a, conf.a
		//
		// TODO: Associate the symbol with schema (ie. getTypeSchema(Type) -> BlockSchema)
		symbols.Define(name, types.NewBlockRefType(kind, name), openBrace)
	}

	var diags []diagnostic.Diagnostic

	// Validate each field present in the block.
	seen := make(map[string]bool, len(body.Assignments))
	for _, assign := range body.Assignments {
		if seen[assign.Name] {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeDuplicateField,
				Position: diagnostic.Position{
					Line:   assign.Start().Line,
					Column: assign.Start().Column,
				},
				Message: fmt.Sprintf("duplicate field %q in %s %q", assign.Name, kind, name),
				Source:  "analyzer",
			})
		}
		seen[assign.Name] = true
		fieldDiags := validateField(assign, name, kind, schema, symbols)
		fieldCodes, fieldAll := suppressedCodes(assign.Annotations)
		fieldDiags = filterSuppressed(fieldDiags, fieldCodes, fieldAll)
		diags = append(diags, fieldDiags...)
	}

	if schema, ok := types.GetSchema(kind); ok {
		if onlyAssignments := helper.HasAnnotation(schema.Annotations, AnnotationOnlyAssignments); onlyAssignments {
			for _, expr := range body.Expressions {
				// TODO: once assignments become expressions, we need to check that here.
				diags = append(diags, diagnostic.Diagnostic{
					Severity: diagnostic.Error,
					Code:     diagnostic.CodeUnexpectedExpr,
					Position: diagnostic.Position{
						Line:   expr.Start().Line,
						Column: expr.Start().Column,
					},
					Message: fmt.Sprintf("unexpected expression in %s block", kind),
					Source:  "analyzer",
				})
			}
		}

	}

	// Validate expressions: only workflow blocks allow bare expressions.
	if kind == BlockKindWorkflow {
		for _, expr := range body.Expressions {
			diags = append(diags, validateWorkflowExpr(expr, symbols)...)
		}
		diags = append(diags, validateTriggerPositions(body.Expressions, symbols)...)
		diags = append(diags, validateWorkflowEntryNodes(name, body.Expressions, symbols)...)
	}

	// Check for missing required fields.
	for fieldName, fieldSchema := range schema.Fields {
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

	exprType := types.ExprType(assign.Value, symbols)
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

	// TODO: This is a "tool" kind specific validation maybe move somewhere else.
	// Validate invoke field specifics for tool blocks.
	if kind == BlockKindTool && assign.Name == "invoke" {
		// TODO: Here we're checking for literal string but first we need to const fold.
		if str, ok := assign.Value.(*ast.StringLiteral); ok {
			if str.Lang != "" && str.Lang != LangTagPython {
				return []diagnostic.Diagnostic{{
					Severity: diagnostic.Warning,
					Code:     diagnostic.CodeUnsupportedLang,
					Position: diagnostic.Position{
						Line:   str.Start().Line,
						Column: str.Start().Column,
					},
					Message: fmt.Sprintf("unsupported language %q in invoke field; only \"py\" is supported", str.Lang),
					Source:  "analyzer",
				}}
			}
			return validateInlineInvoke(str, blockName)
		}
	}

	return nil
}

// validateInlineInvoke checks that an inline Python invoke raw string contains
// a function definition and that the function name matches the tool block name.
// Diagnostics span the entire raw string for visibility.
func validateInlineInvoke(str *ast.StringLiteral, blockName string) []diagnostic.Diagnostic {
	end := str.End()
	rng := diagnostic.Position{Line: str.Start().Line, Column: str.Start().Column}
	endRng := diagnostic.Position{Line: end.EndLine, Column: end.EndCol + 1}

	matches := defNameRe.FindStringSubmatch(str.Value)
	if len(matches) < 2 {
		return []diagnostic.Diagnostic{{
			Severity:    diagnostic.Error,
			Code:        diagnostic.CodeInvalidValue,
			Position:    rng,
			EndPosition: endRng,
			Message:     fmt.Sprintf("invoke raw string must contain a function definition (def %s(...))", blockName),
			Source:      "analyzer",
		}}
	}

	funcName := matches[1]
	if funcName != blockName {
		return []diagnostic.Diagnostic{{
			Severity:    diagnostic.Warning,
			Code:        diagnostic.CodeInvalidValue,
			Position:    rng,
			EndPosition: endRng,
			Message:     fmt.Sprintf("invoke function name %q does not match tool block name %q", funcName, blockName),
			Source:      "analyzer",
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
				Message: fmt.Sprintf("%q has no field %q", objType.BlockKind, e.Member),
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
		diags := analyzeBlockBody(&e.BlockBody, nil, "(inline)", e.TokenStart, e.TokenEnd, symbols)
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
// (agent, tool, cron, webhook) via the type system.
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
	default:
		diags = append(diags, validateWorkflowLeafExpr(expr, symbols)...)
	}
	return diags
}

// validateWorkflowLeafExpr checks a single workflow node position (not an arrow).
// It resolves the expression's type and requires a BlockRef whose kind passes
// IsWorkflowNode(); other types are not valid graph nodes.
func validateWorkflowLeafExpr(expr ast.Expression, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	if refDiags := checkReferences(expr, symbols); len(refDiags) > 0 {
		return refDiags
	}

	// Workflow leaf expression must be a block reference.
	typ := types.ExprType(expr, symbols)
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
	schema, ok := types.GetSchema(typ.BlockKind)
	if !(ok && helper.HasAnnotation(schema.Annotations, AnnotationWorkflowNode)) {
		return []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     diagnostic.CodeInvalidWorkNode,
			Position: diagnostic.Position{
				Line:   expr.Start().Line,
				Column: expr.Start().Column,
			},
			Message: fmt.Sprintf("%s block is not a valid workflow node", typ.BlockKind),
			Source:  "analyzer",
		}}
	}

	return nil
}

// isTriggerExpr returns true if the expression's resolved type is a trigger block kind.
// Works with any expression type (identifiers, member access, subscriptions, etc.)
// by using the type system to infer the block kind.
func isTriggerExpr(expr ast.Expression, symbols *types.SymbolTable) bool {
	typ := types.ExprType(expr, symbols)
	if typ.Kind != types.BlockRef {
		return false
	}
	schema, ok := types.GetSchema(typ.BlockKind)
	if !ok {
		return false
	}
	return helper.HasAnnotation(schema.Annotations, AnnotationTriggerNode)
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

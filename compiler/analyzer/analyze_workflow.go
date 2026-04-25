package analyzer

import (
	"fmt"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
)

func validateWorkflowBlock(body *ast.BlockBody, ap *AnalyzedProgram) []diagnostic.Diagnostic {
	// TODO: This used to be a function we removed it some time ago while doing some refactoring
	// search in commit history to bring this back.
	//
	// validateWorkflowEntryNodes checks the cardinality rules for workflow entry nodes:
	//   - 0 triggers + 2+ entry nodes → error (ambiguous start)
	//   - 1+ triggers + dangling untriggered entry nodes → warning (unreachable)
	// diags = append(diags, validateWorkflowEntryNodes(name, body.Expressions, symbols)...)

	diags := []diagnostic.Diagnostic{}

	// Just to be safe
	if ap == nil {
		return diags
	}

	// Check if "nodes" field is present.
	if nodesExpr, ok := body.GetFieldExpression(types.NodesField); ok {

		// Const fold the nodes expression.
		constVal, foldDiags := constFold(nodesExpr, ap)
		diags = append(diags, foldDiags...)

		// Make sure "nodes" is a const folded map.
		if constVal.Kind != ConstMap {
			start, end := diagnostic.RangeOf(nodesExpr)
			return []diagnostic.Diagnostic{{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeTypeMismatch,
				Position:    start,
				EndPosition: end,
				Message:     "nodes must be a compile time known map",
				Source:      "analyzer",
			}}
		} else {

			// "nodes" keys should be const folded strings.
			for _, key := range constVal.Keys {
				if key.Kind != ConstString {
					start, end := diagnostic.RangeOf(key.Expr)
					diags = append(diags, diagnostic.Diagnostic{
						Severity:    diagnostic.Error,
						Code:        diagnostic.CodeTypeMismatch,
						Position:    start,
						EndPosition: end,
						Message:     "nodes keys must be compile time known strings",
						Source:      "analyzer",
					})
				}
				// TODO: Empty string as key not allowed (maybe other validation like spaces but can allow as well.)
			}

			// "nodes" values should be const folded workflow nodes.
			for _, value := range constVal.Values {
				diags = append(diags, validateWorkflowBlockNodeExpr(body, value.Expr, ap)...)
			}
		}
	}

	// For each expression, validate the workflow expression.
	for _, expr := range body.Expressions {
		diags = append(diags, validateWorkflowExpr(body, expr, ap)...)
	}
	return diags
}

// validateWorkflowExpr checks that a workflow expression only uses the -> operator
// and that each graph endpoint resolves to a workflow-capable block reference
// (agent, tool, cron, webhook, branch) via the type system. When the expression
// resolves to a branch, the branch's route values are recursively validated
// with stricter rules — see validateWorkflowExprRec.
func validateWorkflowExpr(block *ast.BlockBody, expr ast.Expression, ap *AnalyzedProgram) []diagnostic.Diagnostic {
	return validateWorkflowExprRec(block, expr, ap)
}

// validateWorkflowExprRec is the recursive worker for validateWorkflowExpr.
//
// insideRoute indicates the expression is a branch route value. Route values
// follow workflow-expression rules with one extra restriction: triggers are
// not allowed (a trigger can only be the source of a workflow, never a route
// target).
func validateWorkflowExprRec(block *ast.BlockBody, expr ast.Expression, ap *AnalyzedProgram) []diagnostic.Diagnostic {
	if expr == nil {
		return nil
	}
	var diags []diagnostic.Diagnostic

	switch e := expr.(type) {

	// Case 1. Binary expr: <expr> -> <expr>
	case *ast.BinaryExpression:

		// Binary expr `->` is the only valid operator in a workflow block.
		if e.Operator.Type != token.ARROW {
			diags = append(diags, diagnostic.Diagnostic{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeUnexpectedExpr,
				Position:    diagnostic.PositionOf(e.Operator),
				EndPosition: diagnostic.EndPositionOf(e.Operator),
				Message:     fmt.Sprintf("unexpected operator %s in workflow block; only '->' is allowed", token.Describe(e.Operator.Type)),
				Source:      "analyzer",
			})
		}

		diags = append(diags, validateWorkflowExprRec(block, e.Left, ap)...)
		diags = append(diags, validateWorkflowExprRec(block, e.Right, ap)...)

		// Right of -> cannot be a trigger expression.
		if types.IsAnnotated(types.TypeOf(e.Right, ap.SymbolTable), types.AnnotationTriggerNode) {
			start, end := diagnostic.RangeOf(expr)
			diags = append(diags, diagnostic.Diagnostic{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeTriggerAsTarget,
				Position:    start,
				EndPosition: end,
				Message:     "Triggers can only be workflow entry points",
				Source:      "analyzer",
			})
		}

	// Case 2. Leaf expr: <expr>
	default:
		diags = append(diags, validateWorkflowNodeExpr(block, expr, ap)...)
	}
	return diags
}

// nameInNodesMap will return true if the given name is present in the nodes map of the
// workflow block.
func nameInNodesMap(block *ast.BlockBody, name string, ap *AnalyzedProgram) bool {
	nodesMap, ok := block.GetFieldExpression(types.NodesField)
	if !ok {
		return false
	}
	constVal, _ := constFold(nodesMap, ap)
	if constVal.Kind != ConstMap {
		return false
	}
	for _, entry := range constVal.Keys {
		if entry.Kind == ConstString && entry.Str == name {
			return true
		}
	}
	return false
}

// validateWorkflowNodeExpr checks a single workflow node position (not an
// arrow). It requires the expression to resolve to a block annotated with
// @workflow_node. Inline non-branch BlockExpressions are rejected because
// codegen has no path to emit them as workflow nodes (Phase 5 will lift this).
func validateWorkflowNodeExpr(block *ast.BlockBody, expr ast.Expression, ap *AnalyzedProgram) []diagnostic.Diagnostic {

	diags := []diagnostic.Diagnostic{}

	if refDiags := analyzeExpression(expr, ap); len(refDiags) > 0 {
		diags = append(diags, refDiags...)
	}

	// A literal string can be used as a workflow node if and only if it has been registered in the "nodes" map.
	constFolded, foldDiags := constFold(expr, ap)
	diags = append(diags, foldDiags...)
	if constFolded.Kind == ConstString {
		// Make sure the string is in the workflow block "nodes" map.
		if !nameInNodesMap(block, constFolded.Str, ap) {
			start, end := diagnostic.RangeOf(expr)
			diags = append(diags, diagnostic.Diagnostic{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeInvalidWorkNode,
				Position:    start,
				EndPosition: end,
				Message:     fmt.Sprintf("string %s is not a valid workflow node", constFolded.Str),
				Source:      "analyzer",
			})
		}
		return diags
	}

	diags = append(diags, validateWorkflowBlockNodeExpr(block, expr, ap)...)
	return diags
}

// validateWorkflowBlockNodeExpr validates the given expr is a block reference to a workflow node.
// and this doesnt allow string literals (that is a reference to a workflow node).
func validateWorkflowBlockNodeExpr(body *ast.BlockBody, expr ast.Expression, ap *AnalyzedProgram) []diagnostic.Diagnostic {
	typ := types.TypeOf(expr, ap.SymbolTable)
	diags := []diagnostic.Diagnostic{}

	// Make sure the expression resolve to a workflow node
	if !types.IsAnnotated(typ, types.AnnotationWorkflowNode) {
		start, end := diagnostic.RangeOf(expr)
		return []diagnostic.Diagnostic{{
			Severity:    diagnostic.Error,
			Code:        diagnostic.CodeInvalidWorkNode,
			Position:    start,
			EndPosition: end,
			Message:     fmt.Sprintf("%s block is not a valid workflow node", typ.BlockName),
			Source:      "analyzer",
		}}
	}

	// If the name of the block is already present in the nodes map that means we can use this
	// block reference cause we can't implicitly insert.
	//   nodes { "a" : b }
	//   a -> b
	// Here `a` supposed to be implicitly registered as "a":a, but we have "a":b so this cannot
	// be supported.
	//
	// FIXME: This assume no "dynamic block body exists but it could be \() -> new blockbody".
	if blockBody := types.ExprToBlockBody(expr, ap.SymbolTable); blockBody != nil {
		if nameInNodesMap(body, blockBody.Name, ap) {
			start, end := diagnostic.RangeOf(expr)
			return []diagnostic.Diagnostic{{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeInvalidValue,
				Position:    start,
				EndPosition: end,
				Message:     fmt.Sprintf("Name \"%s\" is already present in the nodes map.", blockBody.Name),
				Source:      "analyzer",
			}}
		}
	}

	// If branch, validate the routes.
	if types.IsBlockKind(typ, types.BlockKindBranch) {
		if blockBody := types.ExprToBlockBody(expr, ap.SymbolTable); blockBody != nil {
			diags = append(diags, validateBranchRoutes(body, blockBody, ap)...)
		}
	}

	return diags
}

// validateBranchRoutes validates each route value of a branch as a workflow
// expression with the insideRoute flag set. Used by validateWorkflowExprRec
// when it encounters a branch leaf.
func validateBranchRoutes(wfBody *ast.BlockBody, branchBody *ast.BlockBody, ap *AnalyzedProgram) []diagnostic.Diagnostic {

	routeExpr, ok := branchBody.GetFieldExpression(types.BranchFieldRoute)

	if !ok {
		return nil
	}

	var diags []diagnostic.Diagnostic

	// Constant fold the route expression.
	constVal, foldDiags := constFold(routeExpr, ap)
	diags = append(diags, foldDiags...)

	// route table must be compile time known map.
	if constVal.Kind != ConstMap {
		start, end := diagnostic.RangeOf(routeExpr)
		diags = append(diags, diagnostic.Diagnostic{
			Severity:    diagnostic.Error,
			Code:        diagnostic.CodeTypeMismatch,
			Position:    start,
			EndPosition: end,
			Message:     "route table must be a compile time known map",
			Source:      "analyzer",
		})
		return diags
	}

	// All the route name in the map should be folded to a string.
	for _, entry := range constVal.Keys {
		if entry.Kind != ConstString {
			start, end := diagnostic.RangeOf(entry.Expr)
			diags = append(diags, diagnostic.Diagnostic{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeTypeMismatch,
				Position:    start,
				EndPosition: end,
				Message:     "route name must be a compile time known string",
				Source:      "analyzer",
			})
			continue
		}
	}

	for _, entry := range constVal.Values {
		// Validate the route expression.
		diags = append(diags, validateWorkflowExprRec(wfBody, entry.Expr, ap)...)

		// Inside a branch route, triggers are forbidden — they can only be
		// the source of a workflow, never a route target.
		leftMost := GetLeftMostExpr(entry.Expr)
		if types.IsAnnotated(types.TypeOf(leftMost, ap.SymbolTable), types.AnnotationTriggerNode) {
			start, end := diagnostic.RangeOf(leftMost)
			diags = append(diags, diagnostic.Diagnostic{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeTriggerAsTarget,
				Position:    start,
				EndPosition: end,
				Message:     "Triggers can only be workflow entry points",
				Source:      "analyzer",
			})
		}
	}
	return diags
}

// Return the left most expression in a binary expression or the expression itself if it's not a binary expression.
func GetLeftMostExpr(expr ast.Expression) ast.Expression {
	switch e := expr.(type) {
	case *ast.BinaryExpression:
		return GetLeftMostExpr(e.Left)
	default:
		return expr
	}
}

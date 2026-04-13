package analyzer

import (
	"fmt"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
	"github.com/thakee/orca/compiler/workflow"
)

// validateWorkflowExpr checks that a workflow expression only uses the -> operator
// and that each graph endpoint resolves to a workflow-capable block reference
// (agent, tool, cron, webhook, branch) via the type system. When the expression
// resolves to a branch, the branch's route values are recursively validated
// with stricter rules — see validateWorkflowExprRec.
func validateWorkflowExpr(expr ast.Expression, symbols *types.SymbolTable) []diagnostic.Diagnostic {
	return validateWorkflowExprRec(expr, symbols, make(map[string]bool))
}

// validateWorkflowExprRec is the recursive worker for validateWorkflowExpr.
//
// insideRoute indicates the expression is a branch route value. Route values
// follow workflow-expression rules with one extra restriction: triggers are
// not allowed (a trigger can only be the source of a workflow, never a route
// target).
//
// branchSeen guards recursion through nested branches. A branch's route
// values may themselves resolve to branches whose routes are validated in
// turn; the seen set prevents infinite recursion when branches reference
// each other (a → b → a).
func validateWorkflowExprRec(expr ast.Expression, symbols *types.SymbolTable, seenWorkflowNodes map[string]bool) []diagnostic.Diagnostic {
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

		diags = append(diags, validateWorkflowExprRec(e.Left, symbols, seenWorkflowNodes)...)
		diags = append(diags, validateWorkflowExprRec(e.Right, symbols, seenWorkflowNodes)...)

		// Right of -> cannot be a trigger expression.
		if types.IsAnnotated(types.TypeOf(e.Right, symbols), types.AnnotationTriggerNode) {
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
		diags = append(diags, validateWorkflowNodeExpr(expr, symbols, seenWorkflowNodes)...)
	}
	return diags
}

// validateWorkflowNodeExpr checks a single workflow node position (not an
// arrow). It requires the expression to resolve to a block annotated with
// @workflow_node. Inline non-branch BlockExpressions are rejected because
// codegen has no path to emit them as workflow nodes (Phase 5 will lift this).
func validateWorkflowNodeExpr(expr ast.Expression, symbols *types.SymbolTable, seenWorkflowNodes map[string]bool) []diagnostic.Diagnostic {

	diags := []diagnostic.Diagnostic{}

	if refDiags := analyzeExpression(expr, symbols); len(refDiags) > 0 {
		diags = append(diags, refDiags...)
	}

	typ := types.TypeOf(expr, symbols)

	// Add the block to the seen list
	if typ.Kind == types.BlockRef {
		seenWorkflowNodes[typ.BlockName] = true
	}

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

	// If branch, validate the routes.
	if types.IsBlockKind(typ, types.BlockKindBranch) {
		if blockBody := types.ExprToBlockBody(expr, symbols); blockBody != nil {
			diags = append(diags, validateBranchRoutes(blockBody, symbols, seenWorkflowNodes)...)
		}
	}

	return diags
}

// validateBranchRoutes validates each route value of a branch as a workflow
// expression with the insideRoute flag set. Used by validateWorkflowExprRec
// when it encounters a branch leaf.
func validateBranchRoutes(branchBody *ast.BlockBody, symbols *types.SymbolTable, seenWorkflowNodes map[string]bool) []diagnostic.Diagnostic {

	routeExpr, ok := branchBody.GetFieldExpression(workflow.BranchFieldRoute)

	if !ok {
		return nil
	}

	mapLit, ok := routeExpr.(*ast.MapLiteral)
	if !ok {
		return nil
	}

	var diags []diagnostic.Diagnostic
	for _, entry := range mapLit.Entries {
		// Validate the route expression.
		diags = append(diags, validateWorkflowExprRec(entry.Value, symbols, seenWorkflowNodes)...)

		// Inside a branch route, triggers are forbidden — they can only be
		// the source of a workflow, never a route target.
		leftMost := GetLeftMostExpr(entry.Value)
		if types.IsAnnotated(types.TypeOf(leftMost, symbols), types.AnnotationTriggerNode) {
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

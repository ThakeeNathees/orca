package analyzer

import (
	"fmt"
	"math"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
)

// ConstKind classifies the kind of value produced when an expression
// folds to a compile-time constant.
type ConstKind int

const (
	// ConstUnknown means the expression did not fold to a constant, or
	// the folded shape is not represented by ConstValue.
	ConstUnknown ConstKind = iota
	ConstString
	ConstNumber
	ConstBool
	ConstNull
	ConstList
	ConstMap
	ConstBlock
)

// ConstValue holds a single folded constant. Only the field that matches
// Kind is defined; other fields are zero.
type ConstValue struct {
	Kind ConstKind
	Expr ast.Expression

	Str    string
	Number float64
	Bool   bool

	// BlockKind is the original block kind name (e.g. "model", "agent") when
	// Kind == ConstBlock. Empty for other kinds.
	BlockKind string

	Partial bool // True if the constant is partial like contains unknown values.
	List    []ConstValue
	// Note that we can't use a map[string]ConstValue for maps and blocks because
	// Go map iteration is randomized, so when we codegen iterating the map the
	// order is not deterministic, causing golden test flakiness, and unpredictable
	// codegen output.
	Keys   []string     // Map keys
	Values []ConstValue // Map values
}

// ConstFold performs compile-time constant folding on an expression.
// A zero AnalyzedProgram (nil Symbols and nil AST) folds literals and
// structure only; identifier resolution and block refs need Symbols and AST.
// Returns the folded constant value and any diagnostics.
func ConstFold(expr ast.Expression, ap *AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {

	// If we have cache
	if ap != nil && ap.ConstFoldCache != nil {
		if cached, ok := ap.ConstFoldCache[expr]; ok {
			return cached, nil
		}

		// Cache a sentinel to break cyclic reference cause infinite recursion.
		ap.ConstFoldCache[expr] = ConstValue{Kind: ConstUnknown, Partial: true, Expr: expr}

		constVal, diags := constFold(expr, ap)
		constVal.Expr = expr
		ap.ConstFoldCache[expr] = constVal

		return constVal, diags
	}

	// We dont have a cache, just fold the expression.
	constVal, diags := constFold(expr, ap)
	constVal.Expr = expr
	return constVal, diags
}

func constFold(expr ast.Expression, ap *AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {

	if expr == nil {
		return ConstValue{Kind: ConstUnknown, Partial: true}, nil
	}
	var diags []diagnostic.Diagnostic
	switch e := expr.(type) {
	case *ast.Identifier:
		return foldIdentifier(e, ap)
	case *ast.StringLiteral:
		return ConstValue{Kind: ConstString, Str: e.Value}, diags
	case *ast.NumberLiteral:
		return ConstValue{Kind: ConstNumber, Number: e.Value}, diags
	case *ast.BinaryExpression:
		return foldBinary(e, ap)
	case *ast.MemberAccess:
		return foldMemberAccess(e, ap)
	case *ast.Subscription:
		return foldSubscription(e, ap)
	case *ast.CallExpression:
		return foldCallExpression(e, ap)
	case *ast.Lambda:
		return ConstValue{Kind: ConstUnknown}, diags // Lambdas are not constant-foldable.
	case *ast.MapLiteral:
		return foldMapLiteral(e, ap)
	case *ast.ListLiteral:
		return foldListLiteral(e, ap)
	case *ast.TernaryExpression:
		// TODO: If the condition is a constant, we can fold the ternary.
		return ConstValue{Kind: ConstUnknown}, diags
	case *ast.BlockExpression:
		return foldBlockBody(&e.BlockBody, ap)
	default:
		return ConstValue{Kind: ConstUnknown}, diags
	}
}

// foldMapLiteral builds ConstMap when every entry key is a foldable map key
// (string, identifier, or integer literal) and every value folds to a constant.
func foldMapLiteral(ml *ast.MapLiteral, ap *AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	var diags []diagnostic.Diagnostic
	keys := make([]string, 0, len(ml.Entries))
	values := make([]ConstValue, 0, len(ml.Entries))
	partial := false
	for _, ent := range ml.Entries {
		keyValue, dK := ConstFold(ent.Key, ap)
		valueValue, dV := ConstFold(ent.Value, ap)
		diags = append(diags, dK...)
		diags = append(diags, dV...)
		if keyValue.Kind == ConstUnknown || valueValue.Kind == ConstUnknown {
			partial = true
		}
		if keyValue.Kind != ConstString {
			start, end := diagnostic.RangeOf(ent.Key)
			diags = append(diags, diagnostic.Diagnostic{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeTypeMismatch,
				Position:    start,
				EndPosition: end,
				Message:     fmt.Sprintf("map key must be a string, identifier, or integer, got %T", ent.Key),
				Source:      "analyzer",
			})
		}
		keys = append(keys, keyValue.Str)
		values = append(values, valueValue)
	}
	return ConstValue{Kind: ConstMap, Keys: keys, Values: values, Partial: partial}, diags
}

// foldListLiteral builds ConstList when every element folds to a constant.
func foldListLiteral(ll *ast.ListLiteral, ap *AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	var diags []diagnostic.Diagnostic
	if len(ll.Elements) == 0 {
		return ConstValue{Kind: ConstList, List: []ConstValue{}}, diags
	}
	out := make([]ConstValue, 0, len(ll.Elements))
	partial := false
	for _, el := range ll.Elements {
		v, dV := ConstFold(el, ap)
		diags = append(diags, dV...)
		if v.Kind == ConstUnknown {
			partial = true
		}
		out = append(out, v)
	}
	return ConstValue{Kind: ConstList, List: out, Partial: partial}, diags
}

// foldBlockBody builds ConstBlock when the block body has no workflow edge
// expressions and every assignment value folds to a constant.
func foldBlockBody(body *ast.BlockBody, ap *AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	var diags []diagnostic.Diagnostic
	if len(body.Expressions) > 0 {
		return ConstValue{Kind: ConstBlock, BlockKind: body.Kind, Partial: true}, diags
	}
	keys := make([]string, 0, len(body.Assignments))
	values := make([]ConstValue, 0, len(body.Assignments))
	partial := false
	for _, a := range body.Assignments {
		if a == nil {
			continue
		}
		v, d := ConstFold(a.Value, ap)
		diags = append(diags, d...)
		if v.Kind == ConstUnknown {
			partial = true
		}
		keys = append(keys, a.Name)
		values = append(values, v)
	}
	return ConstValue{Kind: ConstBlock, BlockKind: body.Kind, Keys: keys, Values: values, Partial: partial}, diags
}

// foldBinary folds + - * / on numeric constants, string concatenation for +,
// and leaves workflow operators (->, |) and other cases as unknown.
func foldBinary(e *ast.BinaryExpression, ap *AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	left, d1 := ConstFold(e.Left, ap)
	right, d2 := ConstFold(e.Right, ap)
	diags := append(d1, d2...)
	if left.Kind == ConstUnknown || right.Kind == ConstUnknown {
		return ConstValue{Kind: ConstUnknown}, diags
	}

	switch e.Operator.Type {
	case token.PLUS:
		if left.Kind == ConstString && right.Kind == ConstString {
			return ConstValue{Kind: ConstString, Str: left.Str + right.Str}, diags
		}
		return foldNumericBinary(e.Operator.Type, left, right, diags)
	case token.MINUS, token.STAR, token.SLASH:
		return foldNumericBinary(e.Operator.Type, left, right, diags)
	case token.ARROW, token.PIPE:
		return ConstValue{Kind: ConstUnknown}, diags
	default:
		return ConstValue{Kind: ConstUnknown}, diags
	}
}

// foldNumericBinary applies arithmetic when both operands fold to ConstNumber.
// Values are float64; division uses floating-point / (no separate integer division).
func foldNumericBinary(op token.TokenType, left, right ConstValue, diags []diagnostic.Diagnostic) (ConstValue, []diagnostic.Diagnostic) {
	lhs, lok := constAsNumber(left)
	rhs, rok := constAsNumber(right)

	if lok && rok {
		switch op {
		case token.PLUS:
			return ConstValue{Kind: ConstNumber, Number: lhs + rhs}, diags
		case token.MINUS:
			return ConstValue{Kind: ConstNumber, Number: lhs - rhs}, diags
		case token.STAR:
			return ConstValue{Kind: ConstNumber, Number: lhs * rhs}, diags
		case token.SLASH:
			if rhs == 0 {
				return ConstValue{Kind: ConstUnknown}, diags
			}
			return ConstValue{Kind: ConstNumber, Number: lhs / rhs}, diags
		}
	}

	if !lok || !rok {
		return ConstValue{Kind: ConstUnknown}, diags
	}
	switch op {
	case token.PLUS:
		return ConstValue{Kind: ConstNumber, Number: lhs + rhs}, diags
	case token.MINUS:
		return ConstValue{Kind: ConstNumber, Number: lhs - rhs}, diags
	case token.STAR:
		return ConstValue{Kind: ConstNumber, Number: lhs * rhs}, diags
	case token.SLASH:
		if rhs == 0 {
			return ConstValue{Kind: ConstUnknown}, diags
		}
		return ConstValue{Kind: ConstNumber, Number: lhs / rhs}, diags
	default:
		return ConstValue{Kind: ConstUnknown}, diags
	}
}

// foldIdentifier folds named blocks (including let blocks) by re-folding
// their body. Symbols without a matching block yield ConstUnknown.
func foldIdentifier(e *ast.Identifier, ap *AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	// Parsed as an identifier; does not require symbol resolution for folding.
	if e.Value == types.BlockKindNull {
		return ConstValue{Kind: ConstNull}, nil
	}

	// true and false
	if e.Value == types.BuiltinIdentifierTrue {
		return ConstValue{Kind: ConstBool, Bool: true}, nil
	}
	if e.Value == types.BuiltinIdentifierFalse {
		return ConstValue{Kind: ConstBool, Bool: false}, nil
	}

	if ap == nil || ap.SymbolTable == nil {
		return ConstValue{Kind: ConstUnknown}, nil
	}

	// Check if the identifier is a const fold lambda argument.
	if arg, ok := ap.ConstFoldLambdaArgs[e.Value]; ok {
		return arg, nil
	}

	sym, ok := ap.SymbolTable.LookupSymbol(e.Value)
	if !ok {
		return ConstValue{Kind: ConstUnknown}, nil
	}

	switch sym.Type.Kind {
	case types.BlockRef:
		if ap.Ast == nil {
			return ConstValue{Kind: ConstUnknown}, nil
		}
		if block := ap.Ast.FindBlockWithName(e.Value); block != nil {
			return foldBlockBody(&block.BlockBody, ap)
		}
		return ConstValue{Kind: ConstUnknown}, nil
	default:
		return ConstValue{Kind: ConstUnknown}, nil
	}
}

func findMemberInConstKeyValue(block ConstValue, member string) (ConstValue, bool) {
	for i, key := range block.Keys {
		if key == member {
			return block.Values[i], true
		}
	}
	return ConstValue{Kind: ConstUnknown}, false
}

// foldMemberAccess folds object.member when the object is a constant block-shaped
// value (ConstBlock). Primitives, maps, null, lists, and unknown shapes return
// ConstUnknown (no constant member projection for those yet).
func foldMemberAccess(e *ast.MemberAccess, ap *AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	var diags []diagnostic.Diagnostic
	left, d1 := ConstFold(e.Object, ap)
	diags = append(diags, d1...)

	switch left.Kind {
	case ConstString, ConstNumber, ConstBool:
		// TODO: optional pseudo-members on literals (e.g. string length) if modeled.
		return ConstValue{Kind: ConstUnknown}, diags

	case ConstNull:
		diags = append(diags, diagnostic.Diagnostic{
			Severity:    diagnostic.Error,
			Code:        diagnostic.CodeUnexpectedExpr,
			Position:    diagnostic.PositionOf(e.Dot),
			EndPosition: diagnostic.EndPositionOf(e.Dot),
			Message:     "member access on null",
			Source:      "analyzer",
		})
		return ConstValue{Kind: ConstUnknown}, diags

	case ConstBlock:
		value, ok := findMemberInConstKeyValue(left, e.Member)
		if !ok {
			memberStart, memberEnd := diagnostic.PositionOf(e.End()), diagnostic.EndPositionOf(e.End())
			diags = append(diags, diagnostic.Diagnostic{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeUnknownMember,
				Position:    memberStart,
				EndPosition: memberEnd,
				Message:     fmt.Sprintf("unknown field %q in constant block value", e.Member),
				Source:      "analyzer",
			})
			return ConstValue{Kind: ConstUnknown}, diags
		}
		return value, diags

	default:
		return ConstValue{Kind: ConstUnknown}, diags
	}
}

// foldSubscription returns map[key] for a constant map with string key, or list[i]
// for a constant list with integer index in range. Other shapes or non-constant
// object/index fold to ConstUnknown.
func foldSubscription(e *ast.Subscription, ap *AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	left, d1 := ConstFold(e.Object, ap)
	// Const folding only supports single-index subscriptions (list[i], map[k]).
	if len(e.Indices) != 1 {
		return ConstValue{Kind: ConstUnknown}, d1
	}
	index, d2 := ConstFold(e.Indices[0], ap)
	diags := append(d1, d2...)
	if left.Kind == ConstUnknown || index.Kind == ConstUnknown {
		return ConstValue{Kind: ConstUnknown}, diags
	}
	switch left.Kind {
	case ConstMap:
		idxStart, idxEnd := diagnostic.RangeOf(e.Indices[0])
		if index.Kind != ConstString {
			diags = append(diags, diagnostic.Diagnostic{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeTypeMismatch,
				Position:    idxStart,
				EndPosition: idxEnd,
				Message: fmt.Sprintf(
					"map subscript requires a string key, got constant %s",
					constKindLabel(index.Kind),
				),
				Source: "analyzer",
			})
			return ConstValue{Kind: ConstUnknown}, diags
		}
		value, ok := findMemberInConstKeyValue(left, index.Str)
		if !ok {
			diags = append(diags, diagnostic.Diagnostic{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeUnknownMember,
				Position:    idxStart,
				EndPosition: idxEnd,
				Message:     fmt.Sprintf("unknown map key %q", index.Str),
				Source:      "analyzer",
			})
			return ConstValue{Kind: ConstUnknown}, diags
		}
		return value, diags
	case ConstList:
		idxStart, idxEnd := diagnostic.RangeOf(e.Indices[0])
		if (index.Kind != ConstNumber) || (index.Number != math.Floor(index.Number)) {
			diags = append(diags, diagnostic.Diagnostic{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeInvalidSubscript,
				Position:    idxStart,
				EndPosition: idxEnd,
				Message: fmt.Sprintf(
					"list subscript requires an integer index, got constant %s",
					constKindLabel(index.Kind),
				),
				Source: "analyzer",
			})
			return ConstValue{Kind: ConstUnknown}, diags
		}
		idx := int64(index.Number)
		n := int64(len(left.List))
		if idx < 0 || idx >= n {
			diags = append(diags, diagnostic.Diagnostic{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeInvalidSubscript,
				Position:    idxStart,
				EndPosition: idxEnd,
				Message: fmt.Sprintf(
					"list index %d out of range (length %d)",
					idx, n,
				),
				Source: "analyzer",
			})
			return ConstValue{Kind: ConstUnknown}, diags
		}
		return left.List[idx], diags
	default:
		return ConstValue{Kind: ConstUnknown}, diags
	}
}

func foldCallExpression(e *ast.CallExpression, ap *AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	callee, diags := ConstFold(e.Callee, ap)

	if ap == nil || ap.SymbolTable == nil {
		return ConstValue{Kind: ConstUnknown}, diags
	}

	// Calling lambda function.
	if isConstValueLambda(callee) {

		ap.SymbolTable.PushScope()
		defer ap.SymbolTable.PopScope()

		// Since inside the lambda body we cant use the cache cause it not use the symbol table
		// to resolve the parameter if we use the cache, so we disable cache remproarly.
		savedCache := ap.ConstFoldCache
		ap.ConstFoldCache = nil
		defer func() {
			ap.ConstFoldCache = savedCache
		}()

		// ConstFoldLambdaArgs Also needs a push, pop scope cause a lambda can override the
		// parameter name of another lambda.
		// Example: \(x string) -> \(x -> number) -> x + 1
		// Wait maybe i dont need to do it.

		lambda := callee.Expr.(*ast.Lambda)
		paramCount := len(lambda.Params)
		argCount := len(e.Arguments)

		if paramCount != argCount {
			start, end := diagnostic.RangeOf(e)
			diags = append(diags, diagnostic.Diagnostic{
				Severity:    diagnostic.Error,
				Code:        diagnostic.CodeInvalidArgumentCount,
				Position:    start,
				EndPosition: end,
				Message:     fmt.Sprintf("lambda expects %d arguments, got %d", paramCount, argCount),
				Source:      "analyzer",
			})
			return ConstValue{Kind: ConstUnknown}, diags
		}

		for i, param := range callee.Expr.(*ast.Lambda).Params {
			paramType := types.EvalType(param.TypeExpr, ap.SymbolTable)
			ap.SymbolTable.Define(param.Name.Value, paramType, param.Name.Start())
			arg, argDiags := ConstFold(e.Arguments[i], ap)
			diags = append(diags, argDiags...)
			ap.ConstFoldLambdaArgs[param.Name.Value] = arg
		}

		val, bodyDiags := ConstFold(lambda.Body, ap)
		diags = append(diags, bodyDiags...)

		// Remove the lambda parameters from the const fold lambda args.
		// TODO: This is broken for nested lambdas that reuse a param name — e.g.
		// `(\(x string) -> (\(x number) -> x + 1)(2) + x)("foo")`. When the inner
		// call exits, this delete wipes the outer `x` binding, so the outer body's
		// trailing `+ x` folds to Unknown instead of `"foo"`. Fix: snapshot each
		// param's prior binding on entry and restore (or delete if absent) on exit,
		// instead of always deleting.
		for _, param := range lambda.Params {
			delete(ap.ConstFoldLambdaArgs, param.Name.Value)
		}

		return val, diags
	}

	// TODO: If we reached here = calling to something not lambda.
	return ConstValue{Kind: ConstUnknown}, diags
}

// constKindLabel returns a short name for a folded constant kind (for diagnostic text).
func constKindLabel(k ConstKind) string {
	switch k {
	case ConstString:
		return "string"
	case ConstNumber:
		return "number"
	case ConstBool:
		return "bool"
	case ConstNull:
		return "null"
	case ConstList:
		return "list"
	case ConstMap:
		return "map"
	case ConstBlock:
		return "block"
	default:
		return "unknown"
	}
}

// constAsFloat returns a float64 for int or float folded values.
func constAsNumber(v ConstValue) (float64, bool) {
	switch v.Kind {
	case ConstNumber:
		return v.Number, true
	default:
		return 0, false
	}
}

func isConstValueLambda(v ConstValue) bool {
	if v.Kind != ConstUnknown || v.Expr == nil {
		return false
	}
	_, ok := v.Expr.(*ast.Lambda)
	return ok
}

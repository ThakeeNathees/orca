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
	ConstLambda
)

// ConstValue holds a single folded constant. Only the field that matches
// Kind is defined; other fields are zero.
type ConstValue struct {
	Kind ConstKind
	Expr ast.Expression

	Str    string
	Number float64
	Bool   bool

	// Lambda is the original lambda expression when Kind == ConstLambda.
	Lambda *ast.Lambda

	// BlockKind is the original block kind name (e.g. "model", "agent") when
	// Kind == ConstBlock. Empty for other kinds.
	BlockKind string

	Partial bool // True if the constant is partial like contains unknown values.
	List    []ConstValue
	// Note that we can't use a map[string]ConstValue for maps and blocks because
	// Go map iteration is randomized, so when we codegen iterating the map the
	// order is not deterministic, causing golden test flakiness, and unpredictable
	// codegen output.
	Keys   []ConstValue // Map keys
	Values []ConstValue // Map values
}

// ConstFoldingLambdaArgs is a scope-stack of lambda parameter bindings used
// during constant folding. Each active lambda call pushes a fresh scope,
// defines its params in that scope, folds the body, then pops.
//
// Lookup walks scopes from top to bottom, so inner scopes shadow outer ones
// naturally (and restore them on pop). This is what lets recursive calls —
// e.g. fib(10) → fib(9) → fib(8) — re-use the same param name `n` without
// the inner call wiping the outer's binding.
type ConstFoldingLambdaArgs struct {
	scopes []map[string]ConstValue
}

// NewConstFoldingLambdaArgs returns an empty scope stack.
func NewConstFoldingLambdaArgs() *ConstFoldingLambdaArgs {
	return &ConstFoldingLambdaArgs{}
}

// PushScope starts a new innermost scope for a lambda-call frame.
func (c *ConstFoldingLambdaArgs) PushScope() {
	c.scopes = append(c.scopes, map[string]ConstValue{})
}

// PopScope drops the innermost scope. No-op when empty.
func (c *ConstFoldingLambdaArgs) PopScope() {
	if len(c.scopes) == 0 {
		return
	}
	c.scopes = c.scopes[:len(c.scopes)-1]
}

// Define binds a parameter name to a folded argument value in the innermost
// scope. Silently ignored if no scope has been pushed.
func (c *ConstFoldingLambdaArgs) Define(name string, val ConstValue) {
	if len(c.scopes) == 0 {
		return
	}
	c.scopes[len(c.scopes)-1][name] = val
}

// Lookup searches scopes from innermost to outermost and returns the first
// match. Missing names return the zero value and false. Nil-safe so callers
// holding a bare AnalyzedProgram (tests, LSP snippet folds) don't have to
// initialize the stack just to resolve identifiers that aren't lambda args.
func (c *ConstFoldingLambdaArgs) Lookup(name string) (ConstValue, bool) {
	if c == nil {
		return ConstValue{}, false
	}
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if v, ok := c.scopes[i][name]; ok {
			return v, true
		}
	}
	return ConstValue{}, false
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
		return ConstValue{Kind: ConstLambda, Lambda: e, Expr: e}, diags
	case *ast.MapLiteral:
		return foldMapLiteral(e, ap)
	case *ast.ListLiteral:
		return foldListLiteral(e, ap)
	case *ast.TernaryExpression:
		return foldTernary(e, ap)
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
	keys := make([]ConstValue, 0, len(ml.Entries))
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
		// TODO: Make sure key is hashable.
		keys = append(keys, keyValue)
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

// foldTernary folds the ternary expression.
func foldTernary(e *ast.TernaryExpression, ap *AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	cond, diags := ConstFold(e.Condition, ap)
	if cond.Kind == ConstUnknown {
		return ConstValue{Kind: ConstUnknown}, diags
	}
	if isConstBoolTruthy(cond) {
		return ConstFold(e.TrueExpr, ap)
	}

	return ConstFold(e.FalseExpr, ap)
}

func isConstBoolTruthy(cond ConstValue) bool {
	switch cond.Kind {
	case ConstString:
		return cond.Str != ""
	case ConstNumber:
		return cond.Number != 0
	case ConstBool:
		return cond.Bool
	case ConstNull:
		return false
	case ConstList:
		return len(cond.List) > 0
	case ConstMap:
		return len(cond.Keys) > 0
	case ConstBlock:
		return true
	case ConstLambda:
		return true
	}
	panic(fmt.Sprintf("isConstBoolTruthy: unsupported analyzer.ConstKind %d", cond.Kind))
}

// foldBlockBody builds ConstBlock when the block body has no workflow edge
// expressions and every assignment value folds to a constant.
func foldBlockBody(body *ast.BlockBody, ap *AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	var diags []diagnostic.Diagnostic
	if len(body.Expressions) > 0 {
		return ConstValue{Kind: ConstBlock, BlockKind: body.Kind, Partial: true}, diags
	}
	keys := make([]ConstValue, 0, len(body.Assignments))
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
		keys = append(keys, ConstValue{Kind: ConstString, Str: a.Name})
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
	case token.EQ, token.NEQ:
		// Partial values (lists/maps/blocks containing unknown children) can't
		// be decided at compile time — their runtime shape may differ — so
		// defer to runtime by returning ConstUnknown.
		if left.Partial || right.Partial {
			return ConstValue{Kind: ConstUnknown}, diags
		}
		eq := constValueEqual(left, right)
		if e.Operator.Type == token.NEQ {
			eq = !eq
		}
		return ConstValue{Kind: ConstBool, Bool: eq}, diags
	case token.LT, token.GT, token.LTE, token.GTE:
		// Ordered comparison is only defined for numbers today. String
		// lexicographic comparison can be added later if Orca needs it.
		lhs, lok := constAsNumber(left)
		rhs, rok := constAsNumber(right)
		if !lok || !rok {
			return ConstValue{Kind: ConstUnknown}, diags
		}
		var result bool
		switch e.Operator.Type {
		case token.LT:
			result = lhs < rhs
		case token.GT:
			result = lhs > rhs
		case token.LTE:
			result = lhs <= rhs
		case token.GTE:
			result = lhs >= rhs
		}
		return ConstValue{Kind: ConstBool, Bool: result}, diags
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
	if arg, ok := ap.ConstFoldLambdaArgs.Lookup(e.Value); ok {
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
		if key.Kind == ConstString && key.Str == member {
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

	// Fast path: obj.field where obj is an Identifier naming a top-level
	// BlockStatement. Folding the whole block body (via foldIdentifier →
	// foldBlockBody) would evaluate every sibling assignment, which is both
	// wasteful and a cycle hazard: sibling assignments can reference the same
	// block, causing infinite recursion inside lambda-body evaluation where
	// the cache sentinel is disabled.
	//
	// Instead, look up the matching assignment directly and fold only that one.
	if ident, ok := e.Object.(*ast.Identifier); ok && ap != nil && ap.Ast != nil {
		if block := ap.Ast.FindBlockWithName(ident.Value); block != nil {
			for _, assign := range block.Assignments {
				if assign == nil || assign.Name != e.Member {
					continue
				}
				val, d := ConstFold(assign.Value, ap)
				return val, append(diags, d...)
			}
			// Block exists but field doesn't — emit the same diagnostic the
			// ConstBlock projection path does below so the error surface is
			// consistent across both paths.
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
	}

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

	if ap == nil || ap.SymbolTable == nil || ap.ConstFoldLambdaArgs == nil {
		return ConstValue{Kind: ConstUnknown}, diags
	}

	// Calling lambda function.
	if callee.Kind == ConstLambda {
		ap.SymbolTable.PushScope()
		defer ap.SymbolTable.PopScope()

		// Since inside the lambda body we cant use the cache cause it not use the symbol table
		// to resolve the parameter if we use the cache, so we disable cache remproarly.
		savedCache := ap.ConstFoldCache
		ap.ConstFoldCache = nil
		defer func() {
			ap.ConstFoldCache = savedCache
		}()

		// Push a fresh lambda-args scope for this call. Pop on exit so the
		// caller's bindings are restored — essential for recursion where the
		// inner and outer frames reuse the same param name (e.g. fib's `n`).
		ap.ConstFoldLambdaArgs.PushScope()
		defer ap.ConstFoldLambdaArgs.PopScope()

		lambda := callee.Lambda
		paramCount := 0
		if lambda.Params != nil {
			paramCount = len(lambda.Params)
		}
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

		for i, param := range lambda.Params {
			paramType := types.EvalType(param.TypeExpr, ap.SymbolTable)
			ap.SymbolTable.Define(param.Name.Value, paramType, param.Name.Start())
			arg, argDiags := ConstFold(e.Arguments[i], ap)
			diags = append(diags, argDiags...)
			ap.ConstFoldLambdaArgs.Define(param.Name.Value, arg)
		}

		val, bodyDiags := ConstFold(lambda.Body, ap)
		diags = append(diags, bodyDiags...)

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

// constValueEqual reports semantic equality between two folded constants.
// Callers must filter ConstUnknown and Partial values upfront — both
// foldBinary's `==` path and the tests do — because an unknown operand
// has no defensible compile-time equality answer. This helper assumes
// both inputs are fully determined and compares them by value.
//
// The walk is recursive:
//   - Scalars (string, number, bool, null, blockKind) compared by value.
//   - Lists compared element-wise in order.
//   - Maps and blocks compared by key set — order-insensitive, matching user
//     expectations for `{a:1,b:2} == {b:2,a:1}`.
//
// The Expr field is deliberately skipped at every level: it's metadata for
// the codegen AST-fallback path, not part of the value's semantic identity.
func constValueEqual(a, b ConstValue) bool {
	if a.Kind != b.Kind ||
		a.Str != b.Str ||
		a.Number != b.Number ||
		a.Bool != b.Bool ||
		a.BlockKind != b.BlockKind {
		return false
	}
	if len(a.List) != len(b.List) {
		return false
	}
	for i := range a.List {
		if !constValueEqual(a.List[i], b.List[i]) {
			return false
		}
	}
	if len(a.Keys) != len(b.Keys) || len(a.Values) != len(b.Values) {
		return false
	}

	// Map/block equality is order-insensitive: index b by key, then look up
	// each of a's keys. This matches how users intuitively read `{a:1,b:2}`
	// as a set of named fields rather than an ordered sequence.

	// TODO: This is O(n^2) in the number of keys, we should optimize this (maybe not
	// cause 1. this is compile time and expected not to be large / shouldn't be)
	for idxA, keyA := range a.Keys {
		found := false
		for idxB, keyB := range b.Keys {
			// TODO: We need to add a depth to prevent stack overflow, cause if someone has recursive map
			// that might crash the cosntValueEqual function.
			if constValueEqual(keyA, keyB) {
				if !constValueEqual(a.Values[idxA], b.Values[idxB]) {
					return false
				}
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

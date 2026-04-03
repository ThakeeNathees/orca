package analyzer

import (
	"fmt"

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
	ConstInt
	ConstFloat
	ConstBool
	ConstNull
	ConstList
	ConstMap
	ConstBlock
)

// ConstValue holds a single folded constant. Only the field that matches
// Kind is defined; other fields are zero.
type ConstValue struct {
	Kind  ConstKind
	Str   string
	Int   int64
	Float float64
	Bool  bool

	List     []ConstValue
	KeyValue map[string]ConstValue // Used for maps and blocks
	Partial  bool                  // True if the constant is partial like contains unknown values.
}

// ConstFold performs compile-time constant folding on an expression.
// A zero AnalyzedProgram (nil Symbols and nil AST) folds literals and
// structure only; identifier resolution and block refs need Symbols and AST.
// Returns the folded constant value and any diagnostics.
func ConstFold(expr ast.Expression, ap AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {

	if expr == nil {
		return ConstValue{}, nil
	}
	var diags []diagnostic.Diagnostic
	switch e := expr.(type) {
	case *ast.Identifier:
		return foldIdentifier(e, ap)
	case *ast.StringLiteral:
		return ConstValue{Kind: ConstString, Str: e.Value}, diags
	case *ast.IntegerLiteral:
		return ConstValue{Kind: ConstInt, Int: e.Value}, diags
	case *ast.FloatLiteral:
		return ConstValue{Kind: ConstFloat, Float: e.Value}, diags
	case *ast.BooleanLiteral:
		return ConstValue{Kind: ConstBool, Bool: e.Value}, diags
	case *ast.NullLiteral:
		return ConstValue{Kind: ConstNull}, diags
	case *ast.BinaryExpression:
		return foldBinary(e, ap)
	case *ast.MemberAccess:
		return foldMemberAccess(e, ap)
	case *ast.Subscription:
		return foldSubscription(e, ap)
	case *ast.CallExpression:
		// TODO: fold pure builtins / intrinsics with constant args; else ConstUnknown.
		return ConstValue{Kind: ConstUnknown}, diags
	case *ast.MapLiteral:
		return foldMapLiteral(e, ap)
	case *ast.ListLiteral:
		return foldListLiteral(e, ap)
	case *ast.BlockExpression:
		return foldBlockBody(&e.BlockBody, ap)
	default:
		return ConstValue{Kind: ConstUnknown}, diags
	}
}

// foldMapLiteral builds ConstMap when every entry key is a foldable map key
// (string, identifier, or integer literal) and every value folds to a constant.
func foldMapLiteral(ml *ast.MapLiteral, ap AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	var diags []diagnostic.Diagnostic
	out := make(map[string]ConstValue, len(ml.Entries))
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
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeTypeMismatch,
				Position: diagnostic.Position{Line: ent.Key.Start().Line, Column: ent.Key.Start().Column},
				Message:  fmt.Sprintf("map key must be a string, identifier, or integer, got %T", ent.Key),
				Source:   "analyzer",
			})
		}
		out[keyValue.Str] = valueValue
	}
	return ConstValue{Kind: ConstMap, KeyValue: out, Partial: partial}, diags
}

// foldListLiteral builds ConstList when every element folds to a constant.
func foldListLiteral(ll *ast.ListLiteral, ap AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
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
func foldBlockBody(body *ast.BlockBody, ap AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	var diags []diagnostic.Diagnostic
	if len(body.Expressions) > 0 {
		return ConstValue{Kind: ConstBlock, Partial: true}, diags
	}
	out := make(map[string]ConstValue, len(body.Assignments))
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
		out[a.Name] = v
	}
	return ConstValue{Kind: ConstBlock, KeyValue: out, Partial: partial}, diags
}

// foldBinary folds + - * / on numeric constants, string concatenation for +,
// and leaves workflow operators (->, |) and other cases as unknown.
func foldBinary(e *ast.BinaryExpression, ap AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
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

// foldNumericBinary applies arithmetic when both operands are int or float.
// Mixed int/float promotes to float. Integer division truncates toward zero.
func foldNumericBinary(op token.TokenType, left, right ConstValue, diags []diagnostic.Diagnostic) (ConstValue, []diagnostic.Diagnostic) {
	li, lok := constAsInt(left)
	ri, rok := constAsInt(right)
	lf, lfok := constAsFloat(left)
	rf, rfok := constAsFloat(right)

	if lok && rok {
		switch op {
		case token.PLUS:
			return ConstValue{Kind: ConstInt, Int: li + ri}, diags
		case token.MINUS:
			return ConstValue{Kind: ConstInt, Int: li - ri}, diags
		case token.STAR:
			return ConstValue{Kind: ConstInt, Int: li * ri}, diags
		case token.SLASH:
			if ri == 0 {
				return ConstValue{Kind: ConstUnknown}, diags
			}
			return ConstValue{Kind: ConstInt, Int: li / ri}, diags
		}
	}

	if !lfok || !rfok {
		return ConstValue{Kind: ConstUnknown}, diags
	}
	// Float path (includes mixed int/float via constAsFloat).
	switch op {
	case token.PLUS:
		return ConstValue{Kind: ConstFloat, Float: lf + rf}, diags
	case token.MINUS:
		return ConstValue{Kind: ConstFloat, Float: lf - rf}, diags
	case token.STAR:
		return ConstValue{Kind: ConstFloat, Float: lf * rf}, diags
	case token.SLASH:
		if rf == 0 {
			return ConstValue{Kind: ConstUnknown}, diags
		}
		return ConstValue{Kind: ConstFloat, Float: lf / rf}, diags
	default:
		return ConstValue{Kind: ConstUnknown}, diags
	}
}

// constAsInt reports whether v is a folded integer.
func constAsInt(v ConstValue) (int64, bool) {
	if v.Kind == ConstInt {
		return v.Int, true
	}
	return 0, false
}

// constAsFloat returns a float64 for int or float folded values.
func constAsFloat(v ConstValue) (float64, bool) {
	switch v.Kind {
	case ConstInt:
		return float64(v.Int), true
	case ConstFloat:
		return v.Float, true
	default:
		return 0, false
	}
}

// foldIdentifier folds named blocks (including let blocks) by re-folding
// their body. Symbols without a matching block yield ConstUnknown.
func foldIdentifier(e *ast.Identifier, ap AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	if ap.SymbolTable == nil {
		return ConstValue{Kind: ConstUnknown}, nil
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

// foldMemberAccess folds object.member when the object is a constant block-shaped
// value (ConstBlock). Primitives, maps, null, lists, and unknown shapes return
// ConstUnknown (no constant member projection for those yet).
func foldMemberAccess(e *ast.MemberAccess, ap AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	var diags []diagnostic.Diagnostic
	left, d1 := ConstFold(e.Object, ap)
	diags = append(diags, d1...)

	switch left.Kind {
	case ConstString, ConstInt, ConstFloat, ConstBool:
		// TODO: optional pseudo-members on literals (e.g. string length) if modeled.
		return ConstValue{Kind: ConstUnknown}, diags

	case ConstNull:
		diags = append(diags, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     diagnostic.CodeUnexpectedExpr,
			Position: diagnostic.Position{Line: e.Dot.Line, Column: e.Dot.Column},
			Message:  "member access on null",
			Source:   "analyzer",
		})
		return ConstValue{Kind: ConstUnknown}, diags

	case ConstBlock:
		value, ok := left.KeyValue[e.Member]
		if !ok {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeUnknownMember,
				Position: diagnostic.Position{Line: e.End().Line, Column: e.End().Column},
				Message:  fmt.Sprintf("unknown field %q in constant block value", e.Member),
				Source:   "analyzer",
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
func foldSubscription(e *ast.Subscription, ap AnalyzedProgram) (ConstValue, []diagnostic.Diagnostic) {
	left, d1 := ConstFold(e.Object, ap)
	index, d2 := ConstFold(e.Index, ap)
	diags := append(d1, d2...)
	if left.Kind == ConstUnknown || index.Kind == ConstUnknown {
		return ConstValue{Kind: ConstUnknown}, diags
	}
	switch left.Kind {
	case ConstMap:
		if index.Kind != ConstString {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeTypeMismatch,
				Position: diagnostic.Position{Line: e.Index.Start().Line, Column: e.Index.Start().Column},
				Message: fmt.Sprintf(
					"map subscript requires a string key, got constant %s",
					constKindLabel(index.Kind),
				),
				Source: "analyzer",
			})
			return ConstValue{Kind: ConstUnknown}, diags
		}
		value, ok := left.KeyValue[index.Str]
		if !ok {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeUnknownMember,
				Position: diagnostic.Position{Line: e.Index.Start().Line, Column: e.Index.Start().Column},
				Message:  fmt.Sprintf("unknown map key %q", index.Str),
				Source:   "analyzer",
			})
			return ConstValue{Kind: ConstUnknown}, diags
		}
		return value, diags
	case ConstList:
		if index.Kind != ConstInt {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeInvalidSubscript,
				Position: diagnostic.Position{Line: e.Index.Start().Line, Column: e.Index.Start().Column},
				Message: fmt.Sprintf(
					"list subscript requires an integer index, got constant %s",
					constKindLabel(index.Kind),
				),
				Source: "analyzer",
			})
			return ConstValue{Kind: ConstUnknown}, diags
		}
		idx := index.Int
		n := int64(len(left.List))
		if idx < 0 || idx >= n {
			diags = append(diags, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     diagnostic.CodeInvalidSubscript,
				Position: diagnostic.Position{Line: e.Index.Start().Line, Column: e.Index.Start().Column},
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

// constKindLabel returns a short name for a folded constant kind (for diagnostic text).
func constKindLabel(k ConstKind) string {
	switch k {
	case ConstString:
		return "string"
	case ConstInt:
		return "int"
	case ConstFloat:
		return "float"
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

// primitiveToSchemaType returns the schema of a primitive type.
func primitiveToSchemaType(kind ConstKind) (types.BlockSchema, bool) {
	switch kind {
	case ConstString:
		schema, ok := types.GetSchema("str")
		if !ok {
			return types.BlockSchema{}, false
		}
		return schema, true
	case ConstInt:
		schema, ok := types.GetSchema("int")
		if !ok {
			return types.BlockSchema{}, false
		}
		return schema, true
	case ConstFloat:
		schema, ok := types.GetSchema("float")
		if !ok {
			return types.BlockSchema{}, false
		}
		return schema, true
	case ConstBool:
		schema, ok := types.GetSchema("bool")
		if !ok {
			return types.BlockSchema{}, false
		}
		return schema, true
	case ConstNull:
		schema, ok := types.GetSchema("null")
		if !ok {
			return types.BlockSchema{}, false
		}
		return schema, true
	default:
		return types.BlockSchema{}, false
	}
}

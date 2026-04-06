package types

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

// TestExprType verifies that ExprType returns the correct type for literal expressions.
func TestExprType(t *testing.T) {
	tests := []struct {
		name     string
		expr     ast.Expression
		expected Type
	}{
		{"string literal", &ast.StringLiteral{Value: "hello"}, IdentType("str", nil)},
		{"integer literal", &ast.NumberLiteral{Value: 42}, IdentType("number", nil)},
		{"float literal", &ast.NumberLiteral{Value: 0.5}, IdentType("number", nil)},
		{"boolean true", &ast.BooleanLiteral{Value: true}, IdentType("bool", nil)},
		{"boolean false", &ast.BooleanLiteral{Value: false}, IdentType("bool", nil)},
		{"identifier without symbols resolves as block ref", &ast.Identifier{Value: "gpt4"}, NewBlockRefType("gpt4", nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExprType(tt.expr, nil)
			if got.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", got.Kind)
			}
			if !got.Equals(tt.expected) {
				t.Errorf("ExprType() = %s, want %s", got.String(), tt.expected.String())
			}
		})
	}
}

// TestExprTypeList verifies list literal type inference.
func TestExprTypeList(t *testing.T) {
	got := ExprType(&ast.ListLiteral{}, nil)
	if got.Kind != List {
		t.Fatalf("Kind = %v, want List", got.Kind)
	}

	list := &ast.ListLiteral{
		Elements: []ast.Expression{
			&ast.StringLiteral{Value: "a"},
			&ast.StringLiteral{Value: "b"},
		},
	}
	got = ExprType(list, nil)
	if got.Kind != List {
		t.Fatalf("Kind = %v, want List", got.Kind)
	}
	if got.ElementType != nil {
		t.Fatal("ElementType should be nil until list[T] inference is implemented")
	}
}

// TestExprTypeMapLiteral verifies map literal type inference.
func TestExprTypeMapLiteral(t *testing.T) {
	anyTyp := IdentType("any", nil)
	tests := []struct {
		name    string
		entries []ast.MapEntry
		hasVal  bool
		valTyp  Type
	}{
		{"empty map", nil, false, Type{}},
		{
			"uniform string values",
			[]ast.MapEntry{
				{Key: &ast.Identifier{Value: "a"}, Value: &ast.StringLiteral{Value: "x"}},
				{Key: &ast.Identifier{Value: "b"}, Value: &ast.StringLiteral{Value: "y"}},
			},
			true, anyTyp,
		},
		{
			"uniform int values",
			[]ast.MapEntry{
				{Key: &ast.Identifier{Value: "a"}, Value: &ast.NumberLiteral{Value: 1}},
				{Key: &ast.Identifier{Value: "b"}, Value: &ast.NumberLiteral{Value: 2}},
			},
			true, anyTyp,
		},
		{
			"mixed values",
			[]ast.MapEntry{
				{Key: &ast.Identifier{Value: "a"}, Value: &ast.StringLiteral{Value: "x"}},
				{Key: &ast.Identifier{Value: "b"}, Value: &ast.NumberLiteral{Value: 1}},
			},
			true, anyTyp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ast.MapLiteral{Entries: tt.entries}
			got := ExprType(m, nil)
			if got.Kind != Map {
				t.Errorf("Kind = %v, want Map", got.Kind)
			}
			if tt.hasVal {
				if got.ValueType == nil {
					t.Fatal("ValueType should not be nil")
				}
				if !got.ValueType.Equals(tt.valTyp) {
					t.Errorf("ValueType = %s, want %s", got.ValueType.String(), tt.valTyp.String())
				}
				if got.KeyType == nil || !got.KeyType.Equals(IdentType("str", nil)) {
					t.Error("KeyType should be str")
				}
			} else if got.ValueType != nil {
				t.Error("ValueType should be nil for untyped map")
			}
		})
	}
}

// TestExprTypeIdentWithSymbolTable verifies that identifiers resolve
// to their block reference type when a symbol table is provided.
func TestExprTypeIdentWithSymbolTable(t *testing.T) {
	st := NewSymbolTable()
	st.Define("gpt4", NewBlockRefType("gpt4", nil), token.Token{})
	st.Define("researcher", NewBlockRefType("researcher", nil), token.Token{})
	st.Define("str", NewBlockRefType("str", nil), token.Token{})

	tests := []struct {
		name     string
		ident    string
		expected Type
	}{
		{"defined model", "gpt4", NewBlockRefType("gpt4", nil)},
		{"defined agent", "researcher", NewBlockRefType("researcher", nil)},
		{"builtin schema str", "str", NewBlockRefType("str", nil)},
		{"undefined falls back to NewBlockRefType", "unknown", NewBlockRefType("unknown", nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &ast.Identifier{Value: tt.ident}
			got := ExprType(expr, st)
			if got.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", got.Kind)
			}
			if !got.Equals(tt.expected) {
				t.Errorf("ExprType() = %s, want %s", got.String(), tt.expected.String())
			}
		})
	}
}

// TestExprTypeMemberAccess verifies that member access expressions resolve
// to the field's type via the block schema.
func TestExprTypeMemberAccess(t *testing.T) {
	res := Bootstrap(testBootstrapSource)
	schemaByName := make(map[string]*BlockSchema)
	for i := range res.Schemas {
		s := &res.Schemas[i]
		schemaByName[s.BlockName] = s
	}

	st := NewSymbolTable()
	st.Define("gpt4", NewBlockRefType("gpt4", schemaByName["model"]), token.Token{})

	model := schemaByName["model"]
	tests := []struct {
		name   string
		object string
		member string
		// wantField: if non-empty, compare ExprType to model.Fields[member].Type
		wantField string
	}{
		{"model.provider", "gpt4", "provider", "provider"},
		{"model.temperature", "gpt4", "temperature", "temperature"},
		{"model.model_name", "gpt4", "model_name", "model_name"},
		{"unknown member", "gpt4", "nonexistent", ""},
		{"unknown object", "unknown", "anything", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &ast.MemberAccess{
				Object: &ast.Identifier{Value: tt.object},
				Member: tt.member,
			}
			got := ExprType(expr, st)
			switch {
			case tt.wantField != "":
				want := model.Fields[tt.wantField].Type
				if !got.Equals(want) {
					t.Errorf("ExprType() = %s, want %s", got.String(), want.String())
				}
			case tt.object == "unknown":
				want := IdentType("any", st)
				if !got.Equals(want) {
					t.Errorf("ExprType() = %s, want %s", got.String(), want.String())
				}
			default:
				want := IdentType("any", st)
				if !got.Equals(want) {
					t.Errorf("ExprType() = %s, want %s", got.String(), want.String())
				}
			}
		})
	}
}

// TestExprTypeSubscription verifies that subscription expressions resolve
// to the element type for lists and the value type for maps.
func TestExprTypeSubscription(t *testing.T) {
	anyTyp := IdentType("any", nil)
	tests := []struct {
		name     string
		object   ast.Expression
		index    ast.Expression
		expected Type
	}{
		{
			"list[str] subscript returns any (untyped list)",
			&ast.ListLiteral{Elements: []ast.Expression{
				&ast.StringLiteral{Value: "a"},
				&ast.StringLiteral{Value: "b"},
			}},
			&ast.NumberLiteral{Value: 0},
			anyTyp,
		},
		{
			"list[number] subscript returns any (untyped list)",
			&ast.ListLiteral{Elements: []ast.Expression{
				&ast.NumberLiteral{Value: 1},
				&ast.NumberLiteral{Value: 2},
			}},
			&ast.NumberLiteral{Value: 0},
			anyTyp,
		},
		{
			"map subscript returns any (value type any)",
			&ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: &ast.Identifier{Value: "k"}, Value: &ast.StringLiteral{Value: "v"}},
			}},
			&ast.StringLiteral{Value: "k"},
			anyTyp,
		},
		{
			"untyped list subscript returns any",
			&ast.ListLiteral{},
			&ast.NumberLiteral{Value: 0},
			anyTyp,
		},
		{
			"untyped map subscript returns any",
			&ast.MapLiteral{},
			&ast.StringLiteral{Value: "k"},
			anyTyp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &ast.Subscription{Object: tt.object, Index: tt.index}
			got := ExprType(expr, nil)
			if !got.Equals(tt.expected) {
				t.Errorf("ExprType() = %s, want %s", got.String(), tt.expected.String())
			}
		})
	}
}

// TestExprTypeBinaryArrow verifies that an arrow expression returns any.
func TestExprTypeBinaryArrow(t *testing.T) {
	expr := &ast.BinaryExpression{
		Left:     &ast.Identifier{Value: "a"},
		Operator: token.Token{Type: token.ARROW},
		Right:    &ast.Identifier{Value: "b"},
	}
	got := ExprType(expr, nil)
	want := IdentType("any", nil)
	if !got.Equals(want) {
		t.Errorf("ExprType() = %s, want %s", got.String(), want.String())
	}
}

// TestExprTypeBinaryArithmetic verifies result type inference for arithmetic
// and string binary expressions covering all operator + type combinations.
func TestExprTypeBinaryArithmetic(t *testing.T) {
	numLit := func(v float64) ast.Expression { return &ast.NumberLiteral{Value: v} }
	strLit := func(v string) ast.Expression { return &ast.StringLiteral{Value: v} }
	boolLit := func(v bool) ast.Expression { return &ast.BooleanLiteral{Value: v} }

	strTyp := IdentType("str", nil)
	numTyp := IdentType("number", nil)
	anyTyp := IdentType("any", nil)

	tests := []struct {
		name     string
		op       token.TokenType
		left     ast.Expression
		right    ast.Expression
		expected Type
	}{
		{"str + str → str", token.PLUS, strLit("a"), strLit("b"), strTyp},
		{"number + number → number", token.PLUS, numLit(1), numLit(2), numTyp},
		{"number - number → number", token.MINUS, numLit(3), numLit(1), numTyp},
		{"number * number → number", token.STAR, numLit(2), numLit(4), numTyp},
		{"number / number → number", token.SLASH, numLit(8), numLit(2), numTyp},

		// Mixed / unknown operands → any.
		{"str + number → any", token.PLUS, strLit("a"), numLit(1), anyTyp},
		{"bool + number → any", token.PLUS, boolLit(true), numLit(1), anyTyp},
		{"str - str → any", token.MINUS, strLit("a"), strLit("b"), anyTyp},
		{"str * str → any", token.STAR, strLit("a"), strLit("b"), anyTyp},
		{"str / str → any", token.SLASH, strLit("a"), strLit("b"), anyTyp},

		// Non-arithmetic operators → any.
		{"arrow → any", token.ARROW, numLit(1), numLit(2), anyTyp},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &ast.BinaryExpression{
				Left:     tt.left,
				Operator: token.Token{Type: tt.op},
				Right:    tt.right,
			}
			got := ExprType(expr, nil)
			if !got.Equals(tt.expected) {
				t.Errorf("ExprType() = %s, want %s", got.String(), tt.expected.String())
			}
		})
	}
}

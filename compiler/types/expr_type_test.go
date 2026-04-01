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
		{"string literal", &ast.StringLiteral{Value: "hello"}, Str()},
		{"integer literal", &ast.IntegerLiteral{Value: 42}, Int()},
		{"float literal", &ast.FloatLiteral{Value: 0.5}, Float()},
		{"boolean true", &ast.BooleanLiteral{Value: true}, Bool()},
		{"boolean false", &ast.BooleanLiteral{Value: false}, Bool()},
		{"null literal", &ast.NullLiteral{}, Null()},
		{"identifier without symbols resolves as schema type", &ast.Identifier{Value: "gpt4"}, CreateSchema("gpt4")},
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
	if got.ElementType == nil {
		t.Fatal("ElementType should not be nil")
	}
	if !got.ElementType.Equals(Str()) {
		t.Errorf("ElementType = %s, want str", got.ElementType.String())
	}
}

// TestExprTypeMapLiteral verifies map literal type inference.
func TestExprTypeMapLiteral(t *testing.T) {
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
			true, Str(),
		},
		{
			"uniform int values",
			[]ast.MapEntry{
				{Key: &ast.Identifier{Value: "a"}, Value: &ast.IntegerLiteral{Value: 1}},
				{Key: &ast.Identifier{Value: "b"}, Value: &ast.IntegerLiteral{Value: 2}},
			},
			true, Int(),
		},
		{
			"mixed values",
			[]ast.MapEntry{
				{Key: &ast.Identifier{Value: "a"}, Value: &ast.StringLiteral{Value: "x"}},
				{Key: &ast.Identifier{Value: "b"}, Value: &ast.IntegerLiteral{Value: 1}},
			},
			false, Type{},
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
				if got.KeyType == nil || !got.KeyType.Equals(Str()) {
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
	st.Define("gpt4", NewBlockRefType(token.BlockModel), token.Token{})
	st.Define("researcher", NewBlockRefType(token.BlockAgent), token.Token{})
	st.Define("str", Str(), token.Token{})

	tests := []struct {
		name     string
		ident    string
		expected Type
	}{
		{"defined model", "gpt4", NewBlockRefType(token.BlockModel)},
		{"defined agent", "researcher", NewBlockRefType(token.BlockAgent)},
		{"builtin schema str", "str", Str()},
		{"undefined falls back to schema type", "unknown", CreateSchema("unknown")},
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
	st := NewSymbolTable()
	st.Define("gpt4", NewBlockRefType(token.BlockModel), token.Token{})

	tests := []struct {
		name     string
		object   string
		member   string
		expected Type
		isUnion  bool
	}{
		{"model.provider", "gpt4", "provider", Str(), false},
		{"model.temperature", "gpt4", "temperature", Float(), false},
		{"model.model_name (union)", "gpt4", "model_name", Type{}, true},
		{"unknown member", "gpt4", "nonexistent", Any(), false},
		{"unknown object", "unknown", "anything", Any(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &ast.MemberAccess{
				Object: &ast.Identifier{Value: tt.object},
				Member: tt.member,
			}
			got := ExprType(expr, st)
			if tt.isUnion {
				if got.Kind != Union {
					t.Errorf("Kind = %v, want Union", got.Kind)
				}
			} else {
				if !got.Equals(tt.expected) {
					t.Errorf("ExprType() = %s, want %s", got.String(), tt.expected.String())
				}
			}
		})
	}
}

// TestExprTypeSubscription verifies that subscription expressions resolve
// to the element type for lists and the value type for maps.
func TestExprTypeSubscription(t *testing.T) {
	tests := []struct {
		name     string
		object   ast.Expression
		index    ast.Expression
		expected Type
	}{
		{
			"list[str] subscript returns str",
			&ast.ListLiteral{Elements: []ast.Expression{
				&ast.StringLiteral{Value: "a"},
				&ast.StringLiteral{Value: "b"},
			}},
			&ast.IntegerLiteral{Value: 0},
			Str(),
		},
		{
			"list[int] subscript returns int",
			&ast.ListLiteral{Elements: []ast.Expression{
				&ast.IntegerLiteral{Value: 1},
				&ast.IntegerLiteral{Value: 2},
			}},
			&ast.IntegerLiteral{Value: 0},
			Int(),
		},
		{
			"map[str] subscript returns str",
			&ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: &ast.Identifier{Value: "k"}, Value: &ast.StringLiteral{Value: "v"}},
			}},
			&ast.StringLiteral{Value: "k"},
			Str(),
		},
		{
			"untyped list subscript returns any",
			&ast.ListLiteral{},
			&ast.IntegerLiteral{Value: 0},
			Any(),
		},
		{
			"untyped map subscript returns any",
			&ast.MapLiteral{},
			&ast.StringLiteral{Value: "k"},
			Any(),
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
	if !got.IsAny() {
		t.Errorf("ExprType() = %v, want any", got)
	}
}

// TestExprTypeBinaryArithmetic verifies result type inference for arithmetic
// and string binary expressions covering all operator + type combinations.
func TestExprTypeBinaryArithmetic(t *testing.T) {
	intLit := func(v int64) ast.Expression { return &ast.IntegerLiteral{Value: v} }
	floatLit := func(v float64) ast.Expression { return &ast.FloatLiteral{Value: v} }
	strLit := func(v string) ast.Expression { return &ast.StringLiteral{Value: v} }
	boolLit := func(v bool) ast.Expression { return &ast.BooleanLiteral{Value: v} }

	tests := []struct {
		name     string
		op       token.TokenType
		left     ast.Expression
		right    ast.Expression
		expected Type
	}{
		// String concatenation.
		{"str + str → str", token.PLUS, strLit("a"), strLit("b"), Str()},

		// int op int → int.
		{"int + int → int", token.PLUS, intLit(1), intLit(2), Int()},
		{"int - int → int", token.MINUS, intLit(3), intLit(1), Int()},
		{"int * int → int", token.STAR, intLit(2), intLit(4), Int()},
		{"int / int → int", token.SLASH, intLit(8), intLit(2), Int()},

		// float op float → float.
		{"float + float → float", token.PLUS, floatLit(1.1), floatLit(2.2), Float()},
		{"float - float → float", token.MINUS, floatLit(3.0), floatLit(1.5), Float()},
		{"float * float → float", token.STAR, floatLit(2.0), floatLit(4.0), Float()},
		{"float / float → float", token.SLASH, floatLit(8.0), floatLit(2.0), Float()},

		// int op float → float (numeric widening).
		{"int + float → float", token.PLUS, intLit(1), floatLit(2.5), Float()},
		{"int - float → float", token.MINUS, intLit(5), floatLit(1.5), Float()},
		{"int * float → float", token.STAR, intLit(2), floatLit(3.0), Float()},
		{"int / float → float", token.SLASH, intLit(9), floatLit(3.0), Float()},

		// float op int → float (numeric widening).
		{"float + int → float", token.PLUS, floatLit(2.5), intLit(1), Float()},
		{"float - int → float", token.MINUS, floatLit(5.5), intLit(2), Float()},
		{"float * int → float", token.STAR, floatLit(3.0), intLit(4), Float()},
		{"float / int → float", token.SLASH, floatLit(9.0), intLit(3), Float()},

		// Mixed / unknown operands → any.
		{"str + int → any", token.PLUS, strLit("a"), intLit(1), Any()},
		{"bool + int → any", token.PLUS, boolLit(true), intLit(1), Any()},
		{"str - str → any", token.MINUS, strLit("a"), strLit("b"), Any()},
		{"str * str → any", token.STAR, strLit("a"), strLit("b"), Any()},
		{"str / str → any", token.SLASH, strLit("a"), strLit("b"), Any()},

		// Non-arithmetic operators → any.
		{"arrow → any", token.ARROW, intLit(1), intLit(2), Any()},
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

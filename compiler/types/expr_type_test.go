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
		{"identifier", &ast.Identifier{Value: "gpt4"}, Any()},
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
		name   string
		entries []ast.MapEntry
		hasVal bool
		valTyp Type
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
		{"undefined", "unknown", Any()},
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

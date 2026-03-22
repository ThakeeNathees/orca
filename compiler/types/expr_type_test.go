package types

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

// TestExprType verifies that TypeOf returns the correct type for literal expressions.
func TestExprType(t *testing.T) {
	tests := []struct {
		name     string
		expr     ast.Expression
		expected Type
	}{
		{"string literal", &ast.StringLiteral{Value: "hello"}, StringType},
		{"integer literal", &ast.IntegerLiteral{Value: 42}, IntType},
		{"float literal", &ast.FloatLiteral{Value: 0.5}, FloatType},
		{"boolean true", &ast.BooleanLiteral{Value: true}, BoolType},
		{"boolean false", &ast.BooleanLiteral{Value: false}, BoolType},
		{"list literal", &ast.ListLiteral{}, Type{Kind: List}},
		{"map literal", &ast.MapLiteral{}, Type{Kind: Map}},
		{"identifier", &ast.Identifier{Value: "gpt4"}, AnyType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExprType(tt.expr)
			if got.Kind != tt.expected.Kind {
				t.Errorf("ExprType() Kind = %v, want %v", got.Kind, tt.expected.Kind)
			}
		})
	}
}

// TestExprTypeListElements verifies that a list literal with uniform
// element types produces a typed list.
func TestExprTypeListElements(t *testing.T) {
	list := &ast.ListLiteral{
		Elements: []ast.Expression{
			&ast.StringLiteral{Value: "a"},
			&ast.StringLiteral{Value: "b"},
		},
	}
	got := ExprType(list)
	if got.Kind != List {
		t.Fatalf("Kind = %v, want List", got.Kind)
	}
	if got.ElementType == nil {
		t.Fatal("ElementType should not be nil for uniform list")
	}
	if got.ElementType.Kind != String {
		t.Errorf("ElementType.Kind = %v, want String", got.ElementType.Kind)
	}
}

// TestExprTypeMapLiteral verifies that a map literal with uniform value
// types produces map[T] (key is always string).
func TestExprTypeMapLiteral(t *testing.T) {
	tests := []struct {
		name            string
		entries         []ast.MapEntry
		expectedKind    TypeKind
		expectedValKind *TypeKind
	}{
		{
			"empty map",
			nil,
			Map,
			nil,
		},
		{
			"uniform string values",
			[]ast.MapEntry{
				{Key: &ast.Identifier{Value: "a"}, Value: &ast.StringLiteral{Value: "x"}},
				{Key: &ast.Identifier{Value: "b"}, Value: &ast.StringLiteral{Value: "y"}},
			},
			Map,
			typeKindPtr(String),
		},
		{
			"uniform int values",
			[]ast.MapEntry{
				{Key: &ast.Identifier{Value: "a"}, Value: &ast.IntegerLiteral{Value: 1}},
				{Key: &ast.Identifier{Value: "b"}, Value: &ast.IntegerLiteral{Value: 2}},
			},
			Map,
			typeKindPtr(Int),
		},
		{
			"mixed values",
			[]ast.MapEntry{
				{Key: &ast.Identifier{Value: "a"}, Value: &ast.StringLiteral{Value: "x"}},
				{Key: &ast.Identifier{Value: "b"}, Value: &ast.IntegerLiteral{Value: 1}},
			},
			Map,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ast.MapLiteral{Entries: tt.entries}
			got := ExprType(m)
			if got.Kind != tt.expectedKind {
				t.Errorf("Kind = %v, want %v", got.Kind, tt.expectedKind)
			}
			if tt.expectedValKind != nil {
				if got.ValueType == nil {
					t.Fatalf("ValueType should not be nil")
				}
				if got.ValueType.Kind != *tt.expectedValKind {
					t.Errorf("ValueType.Kind = %v, want %v", got.ValueType.Kind, *tt.expectedValKind)
				}
				// Key is always string.
				if got.KeyType == nil || got.KeyType.Kind != String {
					t.Errorf("KeyType should be String")
				}
			} else if got.ValueType != nil {
				t.Errorf("ValueType should be nil for untyped map")
			}
		})
	}
}

func typeKindPtr(k TypeKind) *TypeKind { return &k }

// TestExprTypeBinaryArrow verifies that an arrow expression returns Any.
func TestExprTypeBinaryArrow(t *testing.T) {
	expr := &ast.BinaryExpression{
		Left:     &ast.Identifier{Value: "a"},
		Operator: token.Token{Type: token.ARROW},
		Right:    &ast.Identifier{Value: "b"},
	}
	got := ExprType(expr)
	if got.Kind != Any {
		t.Errorf("ExprType() Kind = %v, want Any", got.Kind)
	}
}

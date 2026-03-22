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
			got := ExprType(tt.expr, nil)
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
	got := ExprType(list, nil)
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
			got := ExprType(m, nil)
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

// TestExprTypeIdentWithSymbolTable verifies that identifiers resolve
// to their block reference type when a symbol table is provided.
func TestExprTypeIdentWithSymbolTable(t *testing.T) {
	st := NewSymbolTable()
	st.Define("gpt4", NewBlockRefType(BlockModel))
	st.Define("researcher", NewBlockRefType(BlockAgent))

	tests := []struct {
		name      string
		ident     string
		expected  TypeKind
		blockType BlockKind
	}{
		{"defined model", "gpt4", BlockRef, BlockModel},
		{"defined agent", "researcher", BlockRef, BlockAgent},
		{"undefined", "unknown", Any, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &ast.Identifier{Value: tt.ident}
			got := ExprType(expr, st)
			if got.Kind != tt.expected {
				t.Errorf("Kind = %v, want %v", got.Kind, tt.expected)
			}
			if tt.blockType != "" && got.BlockType != tt.blockType {
				t.Errorf("BlockType = %v, want %v", got.BlockType, tt.blockType)
			}
		})
	}
}

// TestExprTypeMemberAccess verifies that member access expressions resolve
// to the field's type via the block schema.
func TestExprTypeMemberAccess(t *testing.T) {
	st := NewSymbolTable()
	st.Define("gpt4", NewBlockRefType(BlockModel))

	tests := []struct {
		name     string
		object   string
		member   string
		expected TypeKind
	}{
		{"model.provider", "gpt4", "provider", String},
		{"model.temperature", "gpt4", "temperature", Float},
		{"model.model_name (union)", "gpt4", "model_name", Union},
		{"unknown member", "gpt4", "nonexistent", Any},
		{"unknown object", "unknown", "anything", Any},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &ast.MemberAccess{
				Object: &ast.Identifier{Value: tt.object},
				Member: tt.member,
			}
			got := ExprType(expr, st)
			if got.Kind != tt.expected {
				t.Errorf("Kind = %v, want %v", got.Kind, tt.expected)
			}
		})
	}
}

// TestExprTypeBinaryArrow verifies that an arrow expression returns Any.
func TestExprTypeBinaryArrow(t *testing.T) {
	expr := &ast.BinaryExpression{
		Left:     &ast.Identifier{Value: "a"},
		Operator: token.Token{Type: token.ARROW},
		Right:    &ast.Identifier{Value: "b"},
	}
	got := ExprType(expr, nil)
	if got.Kind != Any {
		t.Errorf("ExprType() Kind = %v, want Any", got.Kind)
	}
}

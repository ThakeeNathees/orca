package types

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

// TestExprType verifies that ExprType returns the correct type for literal expressions.
func TestExprType(t *testing.T) {
	tests := []struct {
		name      string
		expr      ast.Expression
		blockType BlockKind
	}{
		{"string literal", &ast.StringLiteral{Value: "hello"}, "str"},
		{"integer literal", &ast.IntegerLiteral{Value: 42}, "int"},
		{"float literal", &ast.FloatLiteral{Value: 0.5}, "float"},
		{"boolean true", &ast.BooleanLiteral{Value: true}, "bool"},
		{"boolean false", &ast.BooleanLiteral{Value: false}, "bool"},
		{"null literal", &ast.NullLiteral{}, "null"},
		{"identifier", &ast.Identifier{Value: "gpt4"}, "any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExprType(tt.expr, nil)
			if got.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", got.Kind)
			}
			if got.BlockType != tt.blockType {
				t.Errorf("BlockType = %q, want %q", got.BlockType, tt.blockType)
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
	if got.ElementType.BlockType != "str" {
		t.Errorf("ElementType.BlockType = %q, want %q", got.ElementType.BlockType, "str")
	}
}

// TestExprTypeMapLiteral verifies map literal type inference.
func TestExprTypeMapLiteral(t *testing.T) {
	tests := []struct {
		name   string
		entries []ast.MapEntry
		hasVal bool
		valBT  BlockKind
	}{
		{"empty map", nil, false, ""},
		{
			"uniform string values",
			[]ast.MapEntry{
				{Key: &ast.Identifier{Value: "a"}, Value: &ast.StringLiteral{Value: "x"}},
				{Key: &ast.Identifier{Value: "b"}, Value: &ast.StringLiteral{Value: "y"}},
			},
			true, "str",
		},
		{
			"uniform int values",
			[]ast.MapEntry{
				{Key: &ast.Identifier{Value: "a"}, Value: &ast.IntegerLiteral{Value: 1}},
				{Key: &ast.Identifier{Value: "b"}, Value: &ast.IntegerLiteral{Value: 2}},
			},
			true, "int",
		},
		{
			"mixed values",
			[]ast.MapEntry{
				{Key: &ast.Identifier{Value: "a"}, Value: &ast.StringLiteral{Value: "x"}},
				{Key: &ast.Identifier{Value: "b"}, Value: &ast.IntegerLiteral{Value: 1}},
			},
			false, "",
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
				if got.ValueType.BlockType != tt.valBT {
					t.Errorf("ValueType.BlockType = %q, want %q", got.ValueType.BlockType, tt.valBT)
				}
				if got.KeyType == nil || got.KeyType.BlockType != "str" {
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
	st.Define("gpt4", NewBlockRefType(BlockModel), token.Token{})
	st.Define("researcher", NewBlockRefType(BlockAgent), token.Token{})
	st.Define("str", NewBlockRefType(BlockSchemaKind), token.Token{})

	tests := []struct {
		name      string
		ident     string
		blockType BlockKind
	}{
		{"defined model", "gpt4", BlockModel},
		{"defined agent", "researcher", BlockAgent},
		{"builtin schema str", "str", BlockSchemaKind},
		{"undefined", "unknown", "any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &ast.Identifier{Value: tt.ident}
			got := ExprType(expr, st)
			if got.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", got.Kind)
			}
			if got.BlockType != tt.blockType {
				t.Errorf("BlockType = %q, want %q", got.BlockType, tt.blockType)
			}
		})
	}
}

// TestExprTypeMemberAccess verifies that member access expressions resolve
// to the field's type via the block schema.
func TestExprTypeMemberAccess(t *testing.T) {
	st := NewSymbolTable()
	st.Define("gpt4", NewBlockRefType(BlockModel), token.Token{})

	tests := []struct {
		name      string
		object    string
		member    string
		blockType BlockKind
	}{
		{"model.provider", "gpt4", "provider", "str"},
		{"model.temperature", "gpt4", "temperature", "float"},
		{"model.model_name (union)", "gpt4", "model_name", ""},
		{"unknown member", "gpt4", "nonexistent", "any"},
		{"unknown object", "unknown", "anything", "any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &ast.MemberAccess{
				Object: &ast.Identifier{Value: tt.object},
				Member: tt.member,
			}
			got := ExprType(expr, st)
			if tt.blockType != "" {
				if got.BlockType != tt.blockType {
					t.Errorf("BlockType = %q, want %q", got.BlockType, tt.blockType)
				}
			} else {
				if got.Kind != Union {
					t.Errorf("Kind = %v, want Union", got.Kind)
				}
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

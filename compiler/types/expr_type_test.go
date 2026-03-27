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
		blockKind token.BlockKind
	}{
		{"string literal", &ast.StringLiteral{Value: "hello"}, token.BlockStr},
		{"integer literal", &ast.IntegerLiteral{Value: 42}, token.BlockInt},
		{"float literal", &ast.FloatLiteral{Value: 0.5}, token.BlockFloat},
		{"boolean true", &ast.BooleanLiteral{Value: true}, token.BlockBool},
		{"boolean false", &ast.BooleanLiteral{Value: false}, token.BlockBool},
		{"null literal", &ast.NullLiteral{}, token.BlockNull},
		{"identifier", &ast.Identifier{Value: "gpt4"}, token.BlockAny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExprType(tt.expr, nil)
			if got.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", got.Kind)
			}
			if got.BlockKind != tt.blockKind {
				t.Errorf("BlockKind = %v, want %v", got.BlockKind, tt.blockKind)
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
	if got.ElementType.BlockKind != token.BlockStr {
		t.Errorf("ElementType.BlockKind = %v, want %v", got.ElementType.BlockKind, token.BlockStr)
	}
}

// TestExprTypeMapLiteral verifies map literal type inference.
func TestExprTypeMapLiteral(t *testing.T) {
	tests := []struct {
		name    string
		entries []ast.MapEntry
		hasVal  bool
		valBK   token.BlockKind
	}{
		{"empty map", nil, false, 0},
		{
			"uniform string values",
			[]ast.MapEntry{
				{Key: &ast.Identifier{Value: "a"}, Value: &ast.StringLiteral{Value: "x"}},
				{Key: &ast.Identifier{Value: "b"}, Value: &ast.StringLiteral{Value: "y"}},
			},
			true, token.BlockStr,
		},
		{
			"uniform int values",
			[]ast.MapEntry{
				{Key: &ast.Identifier{Value: "a"}, Value: &ast.IntegerLiteral{Value: 1}},
				{Key: &ast.Identifier{Value: "b"}, Value: &ast.IntegerLiteral{Value: 2}},
			},
			true, token.BlockInt,
		},
		{
			"mixed values",
			[]ast.MapEntry{
				{Key: &ast.Identifier{Value: "a"}, Value: &ast.StringLiteral{Value: "x"}},
				{Key: &ast.Identifier{Value: "b"}, Value: &ast.IntegerLiteral{Value: 1}},
			},
			false, 0,
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
				if got.ValueType.BlockKind != tt.valBK {
					t.Errorf("ValueType.BlockKind = %v, want %v", got.ValueType.BlockKind, tt.valBK)
				}
				if got.KeyType == nil || got.KeyType.BlockKind != token.BlockStr {
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
	st.Define("str", NewBlockRefType(token.BlockSchema), token.Token{})

	tests := []struct {
		name      string
		ident     string
		blockKind token.BlockKind
	}{
		{"defined model", "gpt4", token.BlockModel},
		{"defined agent", "researcher", token.BlockAgent},
		{"builtin schema str", "str", token.BlockSchema},
		{"undefined", "unknown", token.BlockAny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &ast.Identifier{Value: tt.ident}
			got := ExprType(expr, st)
			if got.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", got.Kind)
			}
			if got.BlockKind != tt.blockKind {
				t.Errorf("BlockKind = %v, want %v", got.BlockKind, tt.blockKind)
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
		name      string
		object    string
		member    string
		blockKind token.BlockKind
		isUnion   bool
	}{
		{"model.provider", "gpt4", "provider", token.BlockStr, false},
		{"model.temperature", "gpt4", "temperature", token.BlockFloat, false},
		{"model.model_name (union)", "gpt4", "model_name", 0, true},
		{"unknown member", "gpt4", "nonexistent", token.BlockAny, false},
		{"unknown object", "unknown", "anything", token.BlockAny, false},
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
				if got.BlockKind != tt.blockKind {
					t.Errorf("BlockKind = %v, want %v", got.BlockKind, tt.blockKind)
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
			TypeOf(token.BlockStr),
		},
		{
			"list[int] subscript returns int",
			&ast.ListLiteral{Elements: []ast.Expression{
				&ast.IntegerLiteral{Value: 1},
				&ast.IntegerLiteral{Value: 2},
			}},
			&ast.IntegerLiteral{Value: 0},
			TypeOf(token.BlockInt),
		},
		{
			"map[str] subscript returns str",
			&ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: &ast.Identifier{Value: "k"}, Value: &ast.StringLiteral{Value: "v"}},
			}},
			&ast.StringLiteral{Value: "k"},
			TypeOf(token.BlockStr),
		},
		{
			"untyped list subscript returns any",
			&ast.ListLiteral{},
			&ast.IntegerLiteral{Value: 0},
			TypeOf(token.BlockAny),
		},
		{
			"untyped map subscript returns any",
			&ast.MapLiteral{},
			&ast.StringLiteral{Value: "k"},
			TypeOf(token.BlockAny),
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

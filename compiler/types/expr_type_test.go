package types

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

// TestBlockSchemaTypeOfExpr verifies that BlockSchemaTypeOfExpr returns the
// correct type for literal expressions (depth-0 schema-from-expr).
func TestBlockSchemaTypeOfExpr(t *testing.T) {
	st := bootstrapSymtab(t)
	tests := []struct {
		name     string
		expr     ast.Expression
		expected Type
	}{
		{"string literal", &ast.StringLiteral{Value: "hello"}, IdentType(0, "string", st)},
		{"integer literal", &ast.NumberLiteral{Value: 42}, IdentType(0, "number", st)},
		{"float literal", &ast.NumberLiteral{Value: 0.5}, IdentType(0, "number", st)},
		{"undefined identifier resolves via any", &ast.Identifier{Value: "gpt4"}, IdentType(0, "any", st)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BlockSchemaTypeOfExpr(tt.expr, st)
			if got.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", got.Kind)
			}
			if !got.Equals(tt.expected) {
				t.Errorf("BlockSchemaTypeOfExpr() = %s, want %s", got.String(), tt.expected.String())
			}
		})
	}
}

// TestBlockSchemaTypeOfExprList verifies list literal type inference.
func TestBlockSchemaTypeOfExprList(t *testing.T) {
	st := bootstrapSymtab(t)
	got := BlockSchemaTypeOfExpr(&ast.ListLiteral{}, st)
	if got.Kind != List {
		t.Fatalf("Kind = %v, want List", got.Kind)
	}

	list := &ast.ListLiteral{
		Elements: []ast.Expression{
			&ast.StringLiteral{Value: "a"},
			&ast.StringLiteral{Value: "b"},
		},
	}
	got = BlockSchemaTypeOfExpr(list, st)
	if got.Kind != List {
		t.Fatalf("Kind = %v, want List", got.Kind)
	}
	if got.ElementType != nil {
		t.Fatal("ElementType should be nil until list[T] inference is implemented")
	}
}

// TestBlockSchemaTypeOfExprMapLiteral verifies map literal type inference.
func TestBlockSchemaTypeOfExprMapLiteral(t *testing.T) {
	st := bootstrapSymtab(t)
	anyTyp := IdentType(0, "any", st)
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
			got := BlockSchemaTypeOfExpr(m, st)
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
				if got.KeyType == nil || !got.KeyType.Equals(IdentType(0, "string", st)) {
					t.Error("KeyType should be string (Orca primitive)")
				}
			} else if got.ValueType != nil {
				t.Error("ValueType should be nil for untyped map")
			}
		})
	}
}

// TestBlockSchemaTypeOfExprIdentWithSymbolTable verifies that identifiers resolve
// to their block reference type when a symbol table is provided.
func TestBlockSchemaTypeOfExprIdentWithSymbolTable(t *testing.T) {
	st := NewSymbolTable()
	// anyType() resolves to IdentType("any", …); the table must define "any" or lookups recurse.
	st.Define("any", NewBlockRefType("any", nil), token.Token{})
	st.Define("gpt4", NewBlockRefType("gpt4", nil), token.Token{})
	st.Define("researcher", NewBlockRefType("researcher", nil), token.Token{})
	st.Define("string", NewBlockRefType("string", nil), token.Token{})

	tests := []struct {
		name     string
		ident    string
		expected Type
	}{
		{"defined model", "gpt4", NewBlockRefType("gpt4", nil)},
		{"defined agent", "researcher", NewBlockRefType("researcher", nil)},
		{"builtin schema string", "string", NewBlockRefType("string", nil)},
		{"undefined resolves to any", "unknown", IdentType(0, "any", &st)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &ast.Identifier{Value: tt.ident}
			got := BlockSchemaTypeOfExpr(expr, &st)
			if got.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", got.Kind)
			}
			if !got.Equals(tt.expected) {
				t.Errorf("BlockSchemaTypeOfExpr() = %s, want %s", got.String(), tt.expected.String())
			}
		})
	}
}

// TestBlockSchemaTypeOfExprMemberAccess verifies that member access expressions resolve
// to the field's type via the block schema.
func TestBlockSchemaTypeOfExprMemberAccess(t *testing.T) {
	res := Bootstrap(testBootstrapSource)
	schemaByName := make(map[string]*BlockSchema)
	for i := range res.Schemas {
		s := &res.Schemas[i]
		schemaByName[s.BlockName] = s
	}

	st := NewSymbolTable()
	st.Define("any", NewBlockRefType("any", nil), token.Token{})
	st.Define("gpt4", NewBlockRefType("gpt4", schemaByName["model"]), token.Token{})

	model := schemaByName["model"]
	tests := []struct {
		name   string
		object string
		member string
		// wantField: if non-empty, compare BlockSchemaTypeOfExpr to model.Fields[member].Type
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
			got := BlockSchemaTypeOfExpr(expr, &st)
			switch {
			case tt.wantField != "":
				want := model.Fields[tt.wantField].Type
				if !got.Equals(want) {
					t.Errorf("BlockSchemaTypeOfExpr() = %s, want %s", got.String(), want.String())
				}
			case tt.object == "unknown":
				want := IdentType(0, "any", &st)
				if !got.Equals(want) {
					t.Errorf("BlockSchemaTypeOfExpr() = %s, want %s", got.String(), want.String())
				}
			default:
				want := IdentType(0, "any", &st)
				if !got.Equals(want) {
					t.Errorf("BlockSchemaTypeOfExpr() = %s, want %s", got.String(), want.String())
				}
			}
		})
	}
}

// TestBlockSchemaTypeOfExprSubscription verifies that subscription expressions resolve
// to the element type for lists and the value type for maps.
func TestBlockSchemaTypeOfExprSubscription(t *testing.T) {
	st := bootstrapSymtab(t)
	anyTyp := IdentType(0, "any", st)
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
			got := BlockSchemaTypeOfExpr(expr, st)
			if !got.Equals(tt.expected) {
				t.Errorf("BlockSchemaTypeOfExpr() = %s, want %s", got.String(), tt.expected.String())
			}
		})
	}
}

// TestBlockSchemaTypeOfExprBinaryArrow verifies that an arrow expression returns any.
func TestBlockSchemaTypeOfExprBinaryArrow(t *testing.T) {
	st := bootstrapSymtab(t)
	expr := &ast.BinaryExpression{
		Left:     &ast.Identifier{Value: "a"},
		Operator: token.Token{Type: token.ARROW},
		Right:    &ast.Identifier{Value: "b"},
	}
	got := BlockSchemaTypeOfExpr(expr, st)
	want := IdentType(0, "any", st)
	if !got.Equals(want) {
		t.Errorf("BlockSchemaTypeOfExpr() = %s, want %s", got.String(), want.String())
	}
}

// TestBlockSchemaTypeOfExprBinaryArithmetic verifies result type inference for arithmetic
// and string binary expressions covering all operator + type combinations.
func TestBlockSchemaTypeOfExprBinaryArithmetic(t *testing.T) {
	st := bootstrapSymtab(t)
	numLit := func(v float64) ast.Expression { return &ast.NumberLiteral{Value: v} }
	strLit := func(v string) ast.Expression { return &ast.StringLiteral{Value: v} }

	strTyp := IdentType(0, "string", st)
	numTyp := IdentType(0, "number", st)
	anyTyp := IdentType(0, "any", st)

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
			got := BlockSchemaTypeOfExpr(expr, st)
			if !got.Equals(tt.expected) {
				t.Errorf("BlockSchemaTypeOfExpr() = %s, want %s", got.String(), tt.expected.String())
			}
		})
	}
}

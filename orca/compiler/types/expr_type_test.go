package types

import (
	"testing"

	"github.com/thakee/orca/orca/compiler/ast"
	"github.com/thakee/orca/orca/compiler/lexer"
	"github.com/thakee/orca/orca/compiler/parser"
	"github.com/thakee/orca/orca/compiler/token"
)

// TestExprTypeFromExpr verifies that ExprTypeFromExpr returns the
// correct type for literal expressions (depth-0 schema-from-expr).
func TestExprTypeFromExpr(t *testing.T) {
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
			got := EvalType(tt.expr, st)
			if got.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", got.Kind)
			}
			if !got.Equals(tt.expected) {
				t.Errorf("ExprTypeFromExpr() = %s, want %s", got.String(), tt.expected.String())
			}
		})
	}
}

// TestExprTypeFromExprList verifies list literal type inference.
func TestExprTypeFromExprList(t *testing.T) {
	st := bootstrapSymtab(t)
	stringT := IdentType(0, BlockKindString, st)
	anyT := IdentType(0, BlockKindAny, st)

	// Empty list: no element type.
	got := EvalType(&ast.ListLiteral{}, st)
	if got.Kind != List {
		t.Fatalf("Kind = %v, want List", got.Kind)
	}
	if got.ElementType != nil {
		t.Error("empty list ElementType should be nil")
	}

	// Homogeneous list: element type inferred from entries.
	list := &ast.ListLiteral{
		Elements: []ast.Expression{
			&ast.StringLiteral{Value: "a"},
			&ast.StringLiteral{Value: "b"},
		},
	}
	got = EvalType(list, st)
	if got.Kind != List {
		t.Fatalf("Kind = %v, want List", got.Kind)
	}
	if got.ElementType == nil || !got.ElementType.Equals(stringT) {
		t.Errorf("homogeneous ElementType = %v, want %s", got.ElementType, stringT.String())
	}

	// Heterogeneous list: element type falls back to any.
	mixed := &ast.ListLiteral{
		Elements: []ast.Expression{
			&ast.StringLiteral{Value: "a"},
			&ast.NumberLiteral{Value: 1},
		},
	}
	got = EvalType(mixed, st)
	if got.ElementType == nil || !got.ElementType.Equals(anyT) {
		t.Errorf("heterogeneous ElementType = %v, want %s", got.ElementType, anyT.String())
	}
}

// TestExprTypeFromExprMapLiteral verifies map literal type inference.
// Homogeneous entries flow through as map[K, V] with concrete types;
// heterogeneous entries fall back to any on the non-uniform axis.
func TestExprTypeFromExprMapLiteral(t *testing.T) {
	st := bootstrapSymtab(t)
	stringT := IdentType(0, "string", st)
	numberT := IdentType(0, "number", st)
	anyT := IdentType(0, "any", st)

	tests := []struct {
		name    string
		entries []ast.MapEntry
		hasVal  bool
		keyTyp  Type
		valTyp  Type
	}{
		{"empty map", nil, false, Type{}, Type{}},
		{
			"uniform string values",
			[]ast.MapEntry{
				{Key: &ast.StringLiteral{Value: "a"}, Value: &ast.StringLiteral{Value: "x"}},
				{Key: &ast.StringLiteral{Value: "b"}, Value: &ast.StringLiteral{Value: "y"}},
			},
			true, stringT, stringT,
		},
		{
			"uniform number values",
			[]ast.MapEntry{
				{Key: &ast.StringLiteral{Value: "a"}, Value: &ast.NumberLiteral{Value: 1}},
				{Key: &ast.StringLiteral{Value: "b"}, Value: &ast.NumberLiteral{Value: 2}},
			},
			true, stringT, numberT,
		},
		{
			"mixed values fall back to any",
			[]ast.MapEntry{
				{Key: &ast.StringLiteral{Value: "a"}, Value: &ast.StringLiteral{Value: "x"}},
				{Key: &ast.StringLiteral{Value: "b"}, Value: &ast.NumberLiteral{Value: 1}},
			},
			true, stringT, anyT,
		},
		{
			"mixed keys fall back to any",
			[]ast.MapEntry{
				{Key: &ast.StringLiteral{Value: "a"}, Value: &ast.StringLiteral{Value: "x"}},
				{Key: &ast.NumberLiteral{Value: 1}, Value: &ast.StringLiteral{Value: "y"}},
			},
			true, anyT, stringT,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ast.MapLiteral{Entries: tt.entries}
			got := EvalType(m, st)
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
				if got.KeyType == nil {
					t.Fatal("KeyType should not be nil")
				}
				if !got.KeyType.Equals(tt.keyTyp) {
					t.Errorf("KeyType = %s, want %s", got.KeyType.String(), tt.keyTyp.String())
				}
			} else if got.ValueType != nil {
				t.Error("ValueType should be nil for untyped map")
			}
		})
	}
}

// TestExprTypeFromExprIdentWithSymbolTable verifies that identifiers resolve
// to their block reference type when a symbol table is provided.
func TestExprTypeFromExprIdentWithSymbolTable(t *testing.T) {
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
			got := EvalType(expr, &st)
			if got.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", got.Kind)
			}
			if !got.Equals(tt.expected) {
				t.Errorf("ExprTypeFromExpr() = %s, want %s", got.String(), tt.expected.String())
			}
		})
	}
}

// TestExprTypeFromExprMemberAccess verifies that member access expressions resolve
// to the field's type via the block schema.
func TestExprTypeFromExprMemberAccess(t *testing.T) {
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
		// wantField: if non-empty, compare ExprTypeFromExpr to model.Fields[member].Type
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
			got := EvalType(expr, &st)
			switch {
			case tt.wantField != "":
				want := model.Fields[tt.wantField].Type
				if !got.Equals(want) {
					t.Errorf("ExprTypeFromExpr() = %s, want %s", got.String(), want.String())
				}
			case tt.object == "unknown":
				want := IdentType(0, "any", &st)
				if !got.Equals(want) {
					t.Errorf("ExprTypeFromExpr() = %s, want %s", got.String(), want.String())
				}
			default:
				want := IdentType(0, "any", &st)
				if !got.Equals(want) {
					t.Errorf("ExprTypeFromExpr() = %s, want %s", got.String(), want.String())
				}
			}
		})
	}
}

// TestExprTypeFromExprSubscription verifies that subscription expressions resolve
// to the element type for lists and the value type for maps.
func TestExprTypeFromExprSubscription(t *testing.T) {
	st := bootstrapSymtab(t)
	anyTyp := IdentType(0, "any", st)
	tests := []struct {
		name     string
		object   ast.Expression
		index    ast.Expression
		expected Type
	}{
		{
			"list[string] subscript returns string",
			&ast.ListLiteral{Elements: []ast.Expression{
				&ast.StringLiteral{Value: "a"},
				&ast.StringLiteral{Value: "b"},
			}},
			&ast.NumberLiteral{Value: 0},
			IdentType(0, BlockKindString, st),
		},
		{
			"list[number] subscript returns number",
			&ast.ListLiteral{Elements: []ast.Expression{
				&ast.NumberLiteral{Value: 1},
				&ast.NumberLiteral{Value: 2},
			}},
			&ast.NumberLiteral{Value: 0},
			IdentType(0, BlockKindNumber, st),
		},
		{
			"map[string, string] subscript returns string",
			&ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: &ast.StringLiteral{Value: "k"}, Value: &ast.StringLiteral{Value: "v"}},
			}},
			&ast.StringLiteral{Value: "k"},
			IdentType(0, "string", st),
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
			expr := &ast.Subscription{Object: tt.object, Indices: []ast.Expression{tt.index}}
			got := EvalType(expr, st)
			if !got.Equals(tt.expected) {
				t.Errorf("ExprTypeFromExpr() = %s, want %s", got.String(), tt.expected.String())
			}
		})
	}
}

// TestExprTypeFromExprBinaryArrow verifies that an arrow expression returns any.
func TestExprTypeFromExprBinaryArrow(t *testing.T) {
	st := bootstrapSymtab(t)
	expr := &ast.BinaryExpression{
		Left:     &ast.Identifier{Value: "a"},
		Operator: token.Token{Type: token.ARROW},
		Right:    &ast.Identifier{Value: "b"},
	}
	got := EvalType(expr, st)
	want := IdentType(0, "any", st)
	if !got.Equals(want) {
		t.Errorf("ExprTypeFromExpr() = %s, want %s", got.String(), want.String())
	}
}

// TestExprTypeFromExprBinaryArithmetic verifies result type inference for arithmetic
// and string binary expressions covering all operator + type combinations.
func TestExprTypeFromExprBinaryArithmetic(t *testing.T) {
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
			got := EvalType(expr, st)
			if !got.Equals(tt.expected) {
				t.Errorf("ExprTypeFromExpr() = %s, want %s", got.String(), tt.expected.String())
			}
		})
	}
}

// TestTypeOfSelfReferentialBlock regresses the stack overflow that occurred
// when a block field expression referenced the enclosing block — identType's
// anon-schema synthesis re-entered itself without a cycle guard.
func TestTypeOfSelfReferentialBlock(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		fieldIdx  int
		wantKind  TypeKind
		wantElem  string
	}{
		{
			name:     "member access back into same block",
			src:      "let vars {\n  some_list = [1, 2, 3]\n  val = vars.some_list\n}\n",
			fieldIdx: 1,
			wantKind: List,
			wantElem: BlockKindNumber,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.src, "")
			p := parser.New(l)
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				t.Fatalf("parse errors: %v", p.Errors())
			}

			st := bootstrapSymtab(t)
			block := program.Statements[0].(*ast.BlockStatement)
			schema := NewBlockSchema(block.Annotations, block.Name, &block.BlockBody, st)
			st.Define(block.Name, NewBlockRefType(block.Name, &schema), block.NameToken)

			got := TypeOf(block.BlockBody.Assignments[tt.fieldIdx].Value, st)
			if got.Kind != tt.wantKind {
				t.Fatalf("Kind = %v, want %v (%s)", got.Kind, tt.wantKind, got.String())
			}
			if got.ElementType == nil || got.ElementType.BlockName != tt.wantElem {
				t.Errorf("type = %s, want element %s", got.String(), tt.wantElem)
			}
		})
	}
}

package analyzer

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
)

func TestConstFoldLiterals(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expression
		want ConstValue
	}{
		{
			name: "string",
			expr: &ast.StringLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.STRING, Literal: `"a"`}), Value: "hello"},
			want: ConstValue{Kind: ConstString, Str: "hello"},
		},
		{
			name: "int",
			expr: &ast.NumberLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.NUMBER, Literal: "42"}), Value: 42},
			want: ConstValue{Kind: ConstNumber, Number: 42},
		},
		{
			name: "float",
			expr: &ast.NumberLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.NUMBER, Literal: "0.5"}), Value: 0.5},
			want: ConstValue{Kind: ConstNumber, Number: 0.5},
		},
		{
			name: "null",
			expr: &ast.Identifier{BaseNode: ast.NewTerminal(token.Token{Type: token.IDENT, Literal: "null"}), Value: types.BlockKindNull},
			want: ConstValue{Kind: ConstNull},
		},
		{
			name: "bool true",
			expr: &ast.Identifier{BaseNode: ast.NewTerminal(token.Token{Type: token.IDENT, Literal: "true"}), Value: types.BuiltinIdentifierTrue},
			want: ConstValue{Kind: ConstBool, Bool: true},
		},
		{
			name: "bool false",
			expr: &ast.Identifier{BaseNode: ast.NewTerminal(token.Token{Type: token.IDENT, Literal: "false"}), Value: types.BuiltinIdentifierFalse},
			want: ConstValue{Kind: ConstBool, Bool: false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diags := ConstFold(tt.expr, nil)
			if !constValueEqual(got, tt.want) {
				t.Errorf("ConstFold() = %#v, want %#v", got, tt.want)
			}
			if len(diags) != 0 {
				t.Errorf("unexpected diagnostics: %#v", diags)
			}
		})
	}
}

func TestConstFoldListAndMap(t *testing.T) {
	str := func(s string) *ast.StringLiteral {
		return &ast.StringLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.STRING, Literal: `"x"`}), Value: s}
	}
	i := func(n float64) *ast.NumberLiteral {
		return &ast.NumberLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.NUMBER, Literal: "0"}), Value: n}
	}
	id := func(name string) *ast.Identifier {
		return &ast.Identifier{BaseNode: ast.NewTerminal(token.Token{Type: token.IDENT, Literal: name}), Value: name}
	}

	tests := []struct {
		name string
		expr ast.Expression
		want ConstValue
	}{
		{
			name: "empty list",
			expr: &ast.ListLiteral{Elements: []ast.Expression{}},
			want: ConstValue{Kind: ConstList, List: []ConstValue{}},
		},
		{
			name: "list of ints",
			expr: &ast.ListLiteral{Elements: []ast.Expression{i(1), i(2)}},
			want: ConstValue{Kind: ConstList, List: []ConstValue{{Kind: ConstNumber, Number: 1}, {Kind: ConstNumber, Number: 2}}},
		},
		{
			name: "list with ref fails",
			expr: &ast.ListLiteral{Elements: []ast.Expression{i(1), id("x")}},
			// Unresolved identifiers fold to ConstUnknown per element; the list is still ConstList with Partial set.
			want: ConstValue{Kind: ConstList, List: []ConstValue{{Kind: ConstNumber, Number: 1}, {Kind: ConstUnknown}}, Partial: true},
		},
		{
			name: "empty map",
			expr: &ast.MapLiteral{Entries: []ast.MapEntry{}},
			want: ConstValue{Kind: ConstMap},
		},
		{
			name: "map string keys",
			expr: &ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: str("a"), Value: i(1)},
				{Key: str("b"), Value: str("z")},
			}},
			want: ConstValue{
				Kind:   ConstMap,
				Keys:   []ConstValue{{Kind: ConstString, Str: "a"}, {Kind: ConstString, Str: "b"}},
				Values: []ConstValue{{Kind: ConstNumber, Number: 1}, {Kind: ConstString, Str: "z"}},
			},
		},
		{
			name: "map identifier keys",
			expr: &ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: id("k"), Value: i(7)},
			}},
			// Identifier keys preserve their folded kind.
			want: ConstValue{
				Kind:    ConstMap,
				Keys:    []ConstValue{{Kind: ConstUnknown}},
				Values:  []ConstValue{{Kind: ConstNumber, Number: 7}},
				Partial: true,
			},
		},
		{
			name: "map int key",
			expr: &ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: i(10), Value: str("ten")},
			}},
			// Numeric keys preserve their folded kind.
			want: ConstValue{
				Kind:   ConstMap,
				Keys:   []ConstValue{{Kind: ConstNumber, Number: 10}},
				Values: []ConstValue{{Kind: ConstString, Str: "ten"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ConstFold(tt.expr, nil)
			if !constValueEqual(got, tt.want) {
				t.Errorf("ConstFold() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestConstFoldMapNonStringKeyDiagnostic(t *testing.T) {
	numKey := &ast.NumberLiteral{
		BaseNode: ast.NewTerminal(token.Token{Type: token.NUMBER, Literal: "3.14", Line: 5, Column: 3}),
		Value:    3.14,
	}
	str := &ast.StringLiteral{
		BaseNode: ast.NewTerminal(token.Token{Type: token.STRING, Literal: `"v"`}),
		Value:    "v",
	}
	expr := &ast.MapLiteral{Entries: []ast.MapEntry{
		{Key: numKey, Value: str},
	}}
	got, diags := ConstFold(expr, nil)
	// Non-string keys are preserved as their folded kind.
	wantMap := ConstValue{
		Kind:   ConstMap,
		Keys:   []ConstValue{{Kind: ConstNumber, Number: 3.14}},
		Values: []ConstValue{{Kind: ConstString, Str: "v"}},
	}
	if !constValueEqual(got, wantMap) {
		t.Errorf("expected ConstMap with numeric key, got %#v", got)
	}
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %d", len(diags))
	}
}

func TestConstFoldBlockExpression(t *testing.T) {
	str := func(s string) *ast.StringLiteral {
		return &ast.StringLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.STRING, Literal: `"x"`}), Value: s}
	}
	tests := []struct {
		name string
		be   *ast.BlockExpression
		want ConstValue
	}{
		{
			name: "assignments only",
			be: &ast.BlockExpression{
				BlockBody: ast.BlockBody{
					Kind:        "model",
					Expressions: nil,
					Assignments: []*ast.Assignment{
						{Name: "provider", Value: str("openai")},
					},
				},
			},
			want: ConstValue{
				Kind:      ConstBlock,
				BlockKind: "model",
				Keys:      []ConstValue{{Kind: ConstString, Str: "provider"}},
				Values:    []ConstValue{{Kind: ConstString, Str: "openai"}},
			},
		},
		{
			name: "workflow edges not constant",
			be: &ast.BlockExpression{
				BlockBody: ast.BlockBody{
					Kind:        types.BlockKindWorkflow,
					Expressions: []ast.Expression{idExpr("a")},
					Assignments: []*ast.Assignment{{Name: "x", Value: str("y")}},
				},
			},
			// Any non-empty Expressions marks the block as non-constant shape; result is ConstBlock with Partial.
			want: ConstValue{Kind: ConstBlock, BlockKind: types.BlockKindWorkflow, Partial: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ConstFold(tt.be, nil)
			if !constValueEqual(got, tt.want) {
				t.Errorf("ConstFold() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func idExpr(name string) *ast.Identifier {
	return &ast.Identifier{BaseNode: ast.NewTerminal(token.Token{Type: token.IDENT, Literal: name}), Value: name}
}

// memberAccess builds object.member for const-fold tests (BaseNode is unused by folding).
func memberAccess(obj ast.Expression, member string) *ast.MemberAccess {
	return &ast.MemberAccess{
		Object: obj,
		Dot:    token.Token{Type: token.DOT, Literal: "."},
		Member: member,
	}
}

// subExpr builds object[index] for const-fold tests.
func subExpr(obj, index ast.Expression) *ast.Subscription {
	return &ast.Subscription{Object: obj, Indices: []ast.Expression{index}}
}

// TestConstFoldMemberAccess covers foldMemberAccess: ConstBlock field lookup,
// missing members, non-projectable objects, nested member access on constant blocks,
// and member access after foldIdentifier resolves a block ref.
func TestConstFoldMemberAccess(t *testing.T) {
	str := func(s string) *ast.StringLiteral {
		return &ast.StringLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.STRING, Literal: `"x"`}), Value: s}
	}
	i := func(n float64) *ast.NumberLiteral {
		return &ast.NumberLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.NUMBER, Literal: "0"}), Value: n}
	}

	tests := []struct {
		name         string
		expr         ast.Expression
		symbols      *types.SymbolTable
		program      *ast.Program
		want         ConstValue
		expDiagCodes []string // nil means expect no diagnostics
	}{
		{
			name: "block_expression_known_field",
			expr: memberAccess(&ast.BlockExpression{
				BlockBody: ast.BlockBody{
					Kind:        "model",
					Expressions: nil,
					Assignments: []*ast.Assignment{
						{Name: "provider", Value: str("acme")},
					},
				},
			}, "provider"),
			want: ConstValue{Kind: ConstString, Str: "acme"},
		},
		{
			name: "block_expression_unknown_field",
			expr: memberAccess(&ast.BlockExpression{
				BlockBody: ast.BlockBody{
					Kind:        "model",
					Assignments: []*ast.Assignment{{Name: "x", Value: i(1)}},
				},
			}, "nope"),
			want:         ConstValue{Kind: ConstUnknown},
			expDiagCodes: []string{diagnostic.CodeUnknownMember},
		},
		{
			name: "nested_block_member_access",
			expr: memberAccess(memberAccess(&ast.BlockExpression{
				BlockBody: ast.BlockBody{
					Kind: "model",
					Assignments: []*ast.Assignment{
						{Name: "inner", Value: &ast.BlockExpression{
							BlockBody: ast.BlockBody{
								Kind:        "model",
								Assignments: []*ast.Assignment{{Name: "temperature", Value: i(2)}},
							},
						}},
					},
				},
			}, "inner"), "temperature"),
			want: ConstValue{Kind: ConstNumber, Number: 2},
		},
		{
			name: "string_object_not_folded_member",
			expr: memberAccess(str("hello"), "len"),
			want: ConstValue{Kind: ConstUnknown},
		},
		{
			name: "null_object_not_folded_member",
			expr: memberAccess(&ast.Identifier{
				BaseNode: ast.NewTerminal(token.Token{Type: token.IDENT, Literal: "null"}),
				Value:    types.BlockKindNull,
			}, "x"),
			want:         ConstValue{Kind: ConstUnknown},
			expDiagCodes: []string{diagnostic.CodeUnexpectedExpr},
		},
		{
			name: "list_object_not_folded_member",
			expr: memberAccess(&ast.ListLiteral{Elements: []ast.Expression{i(1)}}, "first"),
			want: ConstValue{Kind: ConstUnknown},
		},
		{
			name: "unresolved_identifier_base",
			expr: memberAccess(idExpr("undef"), "field"),
			want: ConstValue{Kind: ConstUnknown},
		},
		{
			name: "block_ref_identifier_then_member",
			symbols: func() *types.SymbolTable {
				st := types.NewSymbolTable()
				st.Define("gpt", types.NewBlockRefType("gpt", nil), token.Token{Type: token.IDENT, Literal: "gpt"})
				return &st
			}(),
			program: &ast.Program{
				Statements: []ast.Statement{
					&ast.BlockStatement{
						BlockBody: ast.BlockBody{
							Assignments: []*ast.Assignment{
								{Name: "provider", Value: str("openai")},
							},
							Name: "gpt",
						},
					},
				},
			},
			expr: memberAccess(idExpr("gpt"), "provider"),
			want: ConstValue{Kind: ConstString, Str: "openai"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ap := &AnalyzedProgram{Ast: tt.program, SymbolTable: tt.symbols, ConstFoldCache: map[ast.Expression]ConstValue{}}
			got, diags := ConstFold(tt.expr, ap)
			if !constValueEqual(got, tt.want) {
				t.Errorf("ConstFold() = %#v, want %#v", got, tt.want)
			}
			assertConstFoldDiagCodes(t, diags, tt.expDiagCodes)
		})
	}
}

// TestConstFoldSubscription covers foldSubscription: list indexing with bounds and
// int index; map lookup with string key; invalid bases and indices.
func TestConstFoldSubscription(t *testing.T) {
	str := func(s string) *ast.StringLiteral {
		return &ast.StringLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.STRING, Literal: `"x"`}), Value: s}
	}
	i := func(n float64) *ast.NumberLiteral {
		return &ast.NumberLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.NUMBER, Literal: "0"}), Value: n}
	}

	tests := []struct {
		name         string
		expr         ast.Expression
		symbols      *types.SymbolTable
		program      *ast.Program
		want         ConstValue
		expDiagCodes []string
	}{
		{
			name: "list_literal_index_hits",
			expr: subExpr(
				&ast.ListLiteral{Elements: []ast.Expression{i(10), i(20), i(30)}},
				i(1),
			),
			want: ConstValue{Kind: ConstNumber, Number: 20},
		},
		{
			name: "list_literal_index_first",
			expr: subExpr(&ast.ListLiteral{Elements: []ast.Expression{i(7)}}, i(0)),
			want: ConstValue{Kind: ConstNumber, Number: 7},
		},
		{
			name:         "list_index_out_of_range_high",
			expr:         subExpr(&ast.ListLiteral{Elements: []ast.Expression{i(1), i(2)}}, i(2)),
			want:         ConstValue{Kind: ConstUnknown},
			expDiagCodes: []string{diagnostic.CodeInvalidSubscript},
		},
		{
			name:         "list_index_negative",
			expr:         subExpr(&ast.ListLiteral{Elements: []ast.Expression{i(1)}}, i(-1)),
			want:         ConstValue{Kind: ConstUnknown},
			expDiagCodes: []string{diagnostic.CodeInvalidSubscript},
		},
		{
			name:         "list_index_not_int",
			expr:         subExpr(&ast.ListLiteral{Elements: []ast.Expression{i(1)}}, str("0")),
			want:         ConstValue{Kind: ConstUnknown},
			expDiagCodes: []string{diagnostic.CodeInvalidSubscript},
		},
		{
			name: "list_index_unresolved_identifier",
			expr: subExpr(&ast.ListLiteral{Elements: []ast.Expression{i(1), i(2)}}, idExpr("i")),
			want: ConstValue{Kind: ConstUnknown},
		},
		{
			name:         "list_empty_index_zero",
			expr:         subExpr(&ast.ListLiteral{Elements: []ast.Expression{}}, i(0)),
			want:         ConstValue{Kind: ConstUnknown},
			expDiagCodes: []string{diagnostic.CodeInvalidSubscript},
		},
		{
			name: "map_literal_string_key_hits",
			expr: subExpr(&ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: str("k"), Value: str("v")},
			}}, str("k")),
			want: ConstValue{Kind: ConstString, Str: "v"},
		},
		{
			name: "map_literal_missing_key",
			expr: subExpr(&ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: str("only"), Value: i(1)},
			}}, str("other")),
			want:         ConstValue{Kind: ConstUnknown},
			expDiagCodes: []string{diagnostic.CodeUnknownMember},
		},
		{
			name: "map_index_not_string",
			expr: subExpr(&ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: str("a"), Value: i(1)},
			}}, i(0)),
			want:         ConstValue{Kind: ConstUnknown},
			expDiagCodes: []string{diagnostic.CodeTypeMismatch},
		},
		{
			name: "map_index_unresolved_identifier",
			expr: subExpr(&ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: str("a"), Value: i(1)},
			}}, idExpr("key")),
			want: ConstValue{Kind: ConstUnknown},
		},
		{
			name: "string_base_not_subscriptable",
			expr: subExpr(str("ab"), i(0)),
			want: ConstValue{Kind: ConstUnknown},
		},
		{
			name: "block_base_not_subscriptable",
			expr: subExpr(&ast.BlockExpression{
				BlockBody: ast.BlockBody{
					Kind:        "model",
					Assignments: []*ast.Assignment{{Name: "x", Value: i(1)}},
				},
			}, i(0)),
			want: ConstValue{Kind: ConstUnknown},
		},
		{
			name: "unresolved_object",
			expr: subExpr(idExpr("xs"), i(0)),
			want: ConstValue{Kind: ConstUnknown},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ap := &AnalyzedProgram{Ast: tt.program, SymbolTable: tt.symbols, ConstFoldCache: map[ast.Expression]ConstValue{}}
			got, diags := ConstFold(tt.expr, ap)
			if !constValueEqual(got, tt.want) {
				t.Errorf("ConstFold() = %#v, want %#v", got, tt.want)
			}
			assertConstFoldDiagCodes(t, diags, tt.expDiagCodes)
		})
	}
}

// assertConstFoldDiagCodes checks diagnostic codes when exp is nil (expect none) or a list of expected codes in order.
func assertConstFoldDiagCodes(t *testing.T, diags []diagnostic.Diagnostic, exp []string) {
	t.Helper()
	if exp == nil {
		if len(diags) != 0 {
			t.Errorf("unexpected diagnostics: %#v", diags)
		}
		return
	}
	if len(diags) != len(exp) {
		t.Fatalf("got %d diagnostics, want %d: %#v", len(diags), len(exp), diags)
	}
	for i := range exp {
		if diags[i].Code != exp[i] {
			t.Errorf("diags[%d].Code = %q, want %q", i, diags[i].Code, exp[i])
		}
	}
}

func TestConstFoldBinary(t *testing.T) {
	i := func(n float64) *ast.NumberLiteral {
		return &ast.NumberLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.NUMBER, Literal: "0"}), Value: n}
	}
	f := func(x float64) *ast.NumberLiteral {
		return &ast.NumberLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.NUMBER, Literal: "0"}), Value: x}
	}
	s := func(t string) *ast.StringLiteral {
		return &ast.StringLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.STRING, Literal: `"x"`}), Value: t}
	}
	bin := func(op token.TokenType, left, right ast.Expression) *ast.BinaryExpression {
		return &ast.BinaryExpression{
			Left:     left,
			Operator: token.Token{Type: op, Literal: string(op)},
			Right:    right,
		}
	}

	tests := []struct {
		name string
		expr ast.Expression
		want ConstValue
	}{
		{name: "int add", expr: bin(token.PLUS, i(2), i(3)), want: ConstValue{Kind: ConstNumber, Number: 5}},
		{name: "int sub", expr: bin(token.MINUS, i(10), i(4)), want: ConstValue{Kind: ConstNumber, Number: 6}},
		{name: "int mul", expr: bin(token.STAR, i(6), i(7)), want: ConstValue{Kind: ConstNumber, Number: 42}},
		{name: "div", expr: bin(token.SLASH, i(7), i(2)), want: ConstValue{Kind: ConstNumber, Number: 3.5}},
		{name: "int div by zero", expr: bin(token.SLASH, i(1), i(0)), want: ConstValue{Kind: ConstUnknown}},
		{name: "float add", expr: bin(token.PLUS, f(1.5), f(2.5)), want: ConstValue{Kind: ConstNumber, Number: 4}},
		{name: "mixed int float", expr: bin(token.PLUS, i(1), f(2.5)), want: ConstValue{Kind: ConstNumber, Number: 3.5}},
		{name: "string concat", expr: bin(token.PLUS, s("a"), s("b")), want: ConstValue{Kind: ConstString, Str: "ab"}},
		{name: "arrow not folded", expr: bin(token.ARROW, idExpr("a"), idExpr("b")), want: ConstValue{Kind: ConstUnknown}},

		// Equality / inequality on scalars.
		{name: "number eq equal", expr: bin(token.EQ, i(42), i(42)), want: ConstValue{Kind: ConstBool, Bool: true}},
		{name: "number eq not equal", expr: bin(token.EQ, i(1), i(2)), want: ConstValue{Kind: ConstBool, Bool: false}},
		{name: "number neq equal", expr: bin(token.NEQ, i(1), i(1)), want: ConstValue{Kind: ConstBool, Bool: false}},
		{name: "number neq not equal", expr: bin(token.NEQ, i(1), i(2)), want: ConstValue{Kind: ConstBool, Bool: true}},
		{name: "string eq equal", expr: bin(token.EQ, s("hi"), s("hi")), want: ConstValue{Kind: ConstBool, Bool: true}},
		{name: "string eq not equal", expr: bin(token.EQ, s("hi"), s("bye")), want: ConstValue{Kind: ConstBool, Bool: false}},
		{name: "cross kind number vs string", expr: bin(token.EQ, i(1), s("1")), want: ConstValue{Kind: ConstBool, Bool: false}},

		// Equality on lists.
		{
			name: "list eq equal",
			expr: bin(token.EQ,
				&ast.ListLiteral{Elements: []ast.Expression{i(1), i(2), i(3)}},
				&ast.ListLiteral{Elements: []ast.Expression{i(1), i(2), i(3)}}),
			want: ConstValue{Kind: ConstBool, Bool: true},
		},
		{
			name: "list eq different length",
			expr: bin(token.EQ,
				&ast.ListLiteral{Elements: []ast.Expression{i(1), i(2), i(3)}},
				&ast.ListLiteral{Elements: []ast.Expression{i(1), i(2)}}),
			want: ConstValue{Kind: ConstBool, Bool: false},
		},
		{
			name: "list eq different element",
			expr: bin(token.EQ,
				&ast.ListLiteral{Elements: []ast.Expression{i(1), i(2), i(3)}},
				&ast.ListLiteral{Elements: []ast.Expression{i(1), i(9), i(3)}}),
			want: ConstValue{Kind: ConstBool, Bool: false},
		},
		{
			name: "list order matters",
			expr: bin(token.EQ,
				&ast.ListLiteral{Elements: []ast.Expression{i(1), i(2)}},
				&ast.ListLiteral{Elements: []ast.Expression{i(2), i(1)}}),
			want: ConstValue{Kind: ConstBool, Bool: false},
		},
		{
			name: "list neq different",
			expr: bin(token.NEQ,
				&ast.ListLiteral{Elements: []ast.Expression{i(1), i(2)}},
				&ast.ListLiteral{Elements: []ast.Expression{i(1), i(3)}}),
			want: ConstValue{Kind: ConstBool, Bool: true},
		},

		// Equality on maps — order-insensitive.
		{
			name: "map eq same order",
			expr: bin(token.EQ,
				&ast.MapLiteral{Entries: []ast.MapEntry{{Key: s("a"), Value: i(1)}, {Key: s("b"), Value: i(2)}}},
				&ast.MapLiteral{Entries: []ast.MapEntry{{Key: s("a"), Value: i(1)}, {Key: s("b"), Value: i(2)}}}),
			want: ConstValue{Kind: ConstBool, Bool: true},
		},
		{
			name: "map eq reordered keys",
			expr: bin(token.EQ,
				&ast.MapLiteral{Entries: []ast.MapEntry{{Key: s("a"), Value: i(1)}, {Key: s("b"), Value: i(2)}}},
				&ast.MapLiteral{Entries: []ast.MapEntry{{Key: s("b"), Value: i(2)}, {Key: s("a"), Value: i(1)}}}),
			want: ConstValue{Kind: ConstBool, Bool: true},
		},
		{
			name: "map eq missing key",
			expr: bin(token.EQ,
				&ast.MapLiteral{Entries: []ast.MapEntry{{Key: s("a"), Value: i(1)}}},
				&ast.MapLiteral{Entries: []ast.MapEntry{{Key: s("b"), Value: i(1)}}}),
			want: ConstValue{Kind: ConstBool, Bool: false},
		},

		// Equality involving unknown / partial operands — defers to runtime.
		{
			name: "unknown operand yields unknown",
			expr: bin(token.EQ, i(1), idExpr("x")),
			want: ConstValue{Kind: ConstUnknown},
		},
		{
			name: "partial list operand yields unknown",
			expr: bin(token.EQ,
				&ast.ListLiteral{Elements: []ast.Expression{i(1), idExpr("x")}},
				&ast.ListLiteral{Elements: []ast.Expression{i(1), i(2)}}),
			want: ConstValue{Kind: ConstUnknown},
		},

		// Ordered numeric comparisons.
		{name: "lt true", expr: bin(token.LT, i(1), i(2)), want: ConstValue{Kind: ConstBool, Bool: true}},
		{name: "lt false equal", expr: bin(token.LT, i(2), i(2)), want: ConstValue{Kind: ConstBool, Bool: false}},
		{name: "lt false greater", expr: bin(token.LT, i(3), i(2)), want: ConstValue{Kind: ConstBool, Bool: false}},
		{name: "gt true", expr: bin(token.GT, i(3), i(2)), want: ConstValue{Kind: ConstBool, Bool: true}},
		{name: "gt false equal", expr: bin(token.GT, i(2), i(2)), want: ConstValue{Kind: ConstBool, Bool: false}},
		{name: "gt false less", expr: bin(token.GT, i(1), i(2)), want: ConstValue{Kind: ConstBool, Bool: false}},
		{name: "lte true less", expr: bin(token.LTE, i(1), i(2)), want: ConstValue{Kind: ConstBool, Bool: true}},
		{name: "lte true equal", expr: bin(token.LTE, i(2), i(2)), want: ConstValue{Kind: ConstBool, Bool: true}},
		{name: "lte false", expr: bin(token.LTE, i(3), i(2)), want: ConstValue{Kind: ConstBool, Bool: false}},
		{name: "gte true greater", expr: bin(token.GTE, i(3), i(2)), want: ConstValue{Kind: ConstBool, Bool: true}},
		{name: "gte true equal", expr: bin(token.GTE, i(2), i(2)), want: ConstValue{Kind: ConstBool, Bool: true}},
		{name: "gte false", expr: bin(token.GTE, i(1), i(2)), want: ConstValue{Kind: ConstBool, Bool: false}},
		{name: "float comparison", expr: bin(token.LT, f(1.5), f(2.5)), want: ConstValue{Kind: ConstBool, Bool: true}},
		{name: "lt non-numeric operand", expr: bin(token.LT, s("a"), s("b")), want: ConstValue{Kind: ConstUnknown}},
		{name: "gt unknown operand", expr: bin(token.GT, i(1), idExpr("x")), want: ConstValue{Kind: ConstUnknown}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ConstFold(tt.expr, nil)
			if !constValueEqual(got, tt.want) {
				t.Errorf("ConstFold() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestConstFoldIdentifier covers foldIdentifier: nil/missing symbol table, non-BlockRef
// symbols, BlockRef without program or without a matching top-level block, and BlockRef
// that resolves to a folded block body.
func TestConstFoldIdentifier(t *testing.T) {
	str := func(s string) *ast.StringLiteral {
		return &ast.StringLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.STRING, Literal: `"x"`}), Value: s}
	}

	tests := []struct {
		name    string
		id      string
		symbols *types.SymbolTable
		program *ast.Program
		want    ConstValue
	}{
		{
			name:    "nil_symbol_table",
			id:      "foo",
			symbols: nil,
			program: nil,
			want:    ConstValue{Kind: ConstUnknown},
		},
		{
			name: "symbol_not_in_table",
			id:   "missing",
			symbols: func() *types.SymbolTable {
				st := types.NewSymbolTable()
				return &st
			}(),
			program: nil,
			want:    ConstValue{Kind: ConstUnknown},
		},
		{
			name: "non_blockref_symbol_yields_unknown",
			id:   "xs",
			symbols: func() *types.SymbolTable {
				st := types.NewSymbolTable()
				st.Define("xs", types.NewListType(types.NewBlockRefType("number", nil)), token.Token{Type: token.IDENT, Literal: "xs"})
				return &st
			}(),
			program: nil,
			want:    ConstValue{Kind: ConstUnknown},
		},
		{
			name: "blockref_nil_program",
			id:   "m",
			symbols: func() *types.SymbolTable {
				st := types.NewSymbolTable()
				st.Define("m", types.NewBlockRefType("m", nil), token.Token{Type: token.IDENT, Literal: "m"})
				return &st
			}(),
			program: nil,
			want:    ConstValue{Kind: ConstUnknown},
		},
		{
			name: "blockref_no_matching_block",
			id:   "m",
			symbols: func() *types.SymbolTable {
				st := types.NewSymbolTable()
				st.Define("m", types.NewBlockRefType("m", nil), token.Token{Type: token.IDENT, Literal: "m"})
				return &st
			}(),
			program: &ast.Program{},
			want:    ConstValue{Kind: ConstUnknown},
		},
		{
			name: "blockref_wrong_block_name_in_program",
			id:   "wanted",
			symbols: func() *types.SymbolTable {
				st := types.NewSymbolTable()
				st.Define("wanted", types.NewBlockRefType("wanted", nil), token.Token{Type: token.IDENT, Literal: "wanted"})
				return &st
			}(),
			program: &ast.Program{
				Statements: []ast.Statement{
					&ast.BlockStatement{
						BlockBody: ast.BlockBody{
							Assignments: []*ast.Assignment{
								{Name: "x", Value: str("y")},
							},
							Name: "other",
						},
					},
				},
			},
			want: ConstValue{Kind: ConstUnknown},
		},
		{
			name: "blockref_folds_matching_block",
			id:   "gpt",
			symbols: func() *types.SymbolTable {
				st := types.NewSymbolTable()
				st.Define("gpt", types.NewBlockRefType("gpt", nil), token.Token{Type: token.IDENT, Literal: "gpt"})
				return &st
			}(),
			program: &ast.Program{
				Statements: []ast.Statement{
					&ast.BlockStatement{
						BlockBody: ast.BlockBody{
							Assignments: []*ast.Assignment{
								{Name: "provider", Value: str("openai")},
								{Name: "temperature", Value: &ast.NumberLiteral{
									BaseNode: ast.NewTerminal(token.Token{Type: token.NUMBER, Literal: "0.5"}),
									Value:    0.5,
								}},
							},
							Name: "gpt",
						},
					},
				},
			},
			want: ConstValue{
				Kind: ConstBlock,
				Keys: []ConstValue{{Kind: ConstString, Str: "provider"}, {Kind: ConstString, Str: "temperature"}},
				Values: []ConstValue{
					{Kind: ConstString, Str: "openai"},
					{Kind: ConstNumber, Number: 0.5},
				},
			},
		},
		{
			name: "blockref_workflow_edges_partial",
			id:   "wf",
			symbols: func() *types.SymbolTable {
				st := types.NewSymbolTable()
				st.Define("wf", types.NewBlockRefType("wf", nil), token.Token{Type: token.IDENT, Literal: "wf"})
				return &st
			}(),
			program: &ast.Program{
				Statements: []ast.Statement{
					&ast.BlockStatement{
						BlockBody: ast.BlockBody{
							Expressions: []ast.Expression{idExpr("a")},
							Assignments: []*ast.Assignment{
								{Name: "x", Value: str("y")},
							},
							Name: "wf",
						},
					},
				},
			},
			want: ConstValue{Kind: ConstBlock, Partial: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ap := &AnalyzedProgram{Ast: tt.program, SymbolTable: tt.symbols, ConstFoldCache: map[ast.Expression]ConstValue{}}
			got, diags := ConstFold(idExpr(tt.id), ap)
			if !constValueEqual(got, tt.want) {
				t.Errorf("ConstFold() = %#v, want %#v", got, tt.want)
			}
			if len(diags) != 0 {
				t.Errorf("unexpected diagnostics: %#v", diags)
			}
		})
	}
}

// TestConstFoldLetBoundProviderMember verifies named let block member access
// folds correctly: config.defaults.provider resolves through the let block body
// then through the inline model block to the string "openai".
// TestConstFoldLetBoundProviderMember is an integration test: it fails until Analyze() accepts the let/model
// program without spurious type-mismatch diagnostics on str fields.
func TestConstFoldLetBoundProviderMember(t *testing.T) {
	input := `let vars {
  defaults = model {
    provider = "openai"
    model_name = "template"
  }
}
model gpt {
  provider = vars.defaults.provider
  model_name = "gpt-4o"
}`
	prog := parseProgram(t, input)
	ap := Analyze(prog)
	if len(ap.Diagnostics) > 0 {
		t.Fatalf("Analyze: %v", ap.Diagnostics)
	}
	gpt := prog.FindBlockWithName("gpt")
	if gpt == nil {
		t.Fatal("model gpt not found")
	}
	expr, ok := gpt.GetFieldExpression("provider")
	if !ok {
		t.Fatal("provider field missing")
	}
	v, diags := ConstFold(expr, &ap)
	if len(diags) != 0 {
		t.Errorf("unexpected diags: %v", diags)
	}
	want := ConstValue{Kind: ConstString, Str: "openai"}
	if !constValueEqual(v, want) {
		t.Errorf("ConstFold(provider) = %#v, want %#v", v, want)
	}
}

// TestConstFoldRecursiveLambdaCall verifies that a recursive lambda call with
// a constant argument folds to the final numeric result at compile time.
// Concretely, `vars.fib(10)` should fold to 55 via repeated lambda-body
// evaluation, exercising: member access through a let block, lambda callee
// resolution, constant-argument binding, ternary condition folding on `>`,
// numeric binary folding, and recursive self-call through `vars.fib`.
func TestConstFoldRecursiveLambdaCall(t *testing.T) {
	input := `let vars {
  fib = \(n number) number ->
    (n > 1) ? vars.fib(n-1) + vars.fib(n-2) : n

  f10 = vars.fib(10)
}`
	prog := parseProgram(t, input)
	ap := Analyze(prog)
	if len(ap.Diagnostics) > 0 {
		t.Fatalf("Analyze: %v", ap.Diagnostics)
	}
	vars := prog.FindBlockWithName("vars")
	if vars == nil {
		t.Fatal("let vars not found")
	}
	expr, ok := vars.GetFieldExpression("f10")
	if !ok {
		t.Fatal("f10 field missing")
	}
	got, diags := ConstFold(expr, &ap)
	if len(diags) != 0 {
		t.Errorf("unexpected diags: %v", diags)
	}
	want := ConstValue{Kind: ConstNumber, Number: 55}
	if !constValueEqual(got, want) {
		t.Errorf("ConstFold(f10) = %#v, want %#v", got, want)
	}
}

// TestConstFoldCacheReuse verifies that ConstFold returns cached results on a
// second call for the same expression without recomputing. We poison the cache
// entry with a sentinel value after the first fold; the second fold must return
// the poisoned value, proving it came from the cache rather than being re-walked.
func TestConstFoldCacheReuse(t *testing.T) {
	expr := &ast.NumberLiteral{
		BaseNode: ast.NewTerminal(token.Token{Type: token.NUMBER, Literal: "42"}),
		Value:    42,
	}
	ap := &AnalyzedProgram{ConstFoldCache: map[ast.Expression]ConstValue{}}

	first, _ := ConstFold(expr, ap)
	if !constValueEqual(first, ConstValue{Kind: ConstNumber, Number: 42}) {
		t.Fatalf("first fold = %#v, want ConstNumber 42", first)
	}
	if _, ok := ap.ConstFoldCache[expr]; !ok {
		t.Fatal("expected expression to be cached after first fold")
	}

	// Poison the cache entry with an obviously wrong value.
	ap.ConstFoldCache[expr] = ConstValue{Kind: ConstString, Str: "poisoned"}

	second, _ := ConstFold(expr, ap)
	if !constValueEqual(second, ConstValue{Kind: ConstString, Str: "poisoned"}) {
		t.Errorf("second fold recomputed instead of using cache: got %#v", second)
	}
}

// TestConstFoldPreservesKeyOrder verifies that map and block folding preserve
// source order in the Keys slice. This is a regression test for a bug where
// ConstValue used a map[string]ConstValue under the hood, causing golden
// codegen output to flake due to Go's randomized map iteration.
func TestConstFoldPreservesKeyOrder(t *testing.T) {
	str := func(s string) *ast.StringLiteral {
		return &ast.StringLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.STRING, Literal: `"x"`}), Value: s}
	}
	i := func(n float64) *ast.NumberLiteral {
		return &ast.NumberLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.NUMBER, Literal: "0"}), Value: n}
	}

	t.Run("map literal", func(t *testing.T) {
		// Use keys whose natural alphabetical ordering differs from source order,
		// so a map-based implementation would flake (and sorted output would
		// visibly reorder to a, b, c, d).
		expr := &ast.MapLiteral{Entries: []ast.MapEntry{
			{Key: str("d"), Value: i(1)},
			{Key: str("b"), Value: i(2)},
			{Key: str("a"), Value: i(3)},
			{Key: str("c"), Value: i(4)},
		}}
		got, _ := ConstFold(expr, nil)
		wantKeys := []ConstValue{{Kind: ConstString, Str: "d"}, {Kind: ConstString, Str: "b"}, {Kind: ConstString, Str: "a"}, {Kind: ConstString, Str: "c"}}
		if len(got.Keys) != len(wantKeys) {
			t.Fatalf("Keys length = %d, want %d", len(got.Keys), len(wantKeys))
		}
		for idx, k := range wantKeys {
			if !constValueEqual(got.Keys[idx], k) {
				t.Errorf("Keys[%d] = %#v, want %#v", idx, got.Keys[idx], k)
			}
		}
	})

	t.Run("block body", func(t *testing.T) {
		be := &ast.BlockExpression{BlockBody: ast.BlockBody{
			Kind: "model",
			Assignments: []*ast.Assignment{
				{Name: "provider", Value: str("openai")},
				{Name: "model_name", Value: str("gpt-4o")},
				{Name: "temperature", Value: i(0.7)},
			},
		}}
		got, _ := ConstFold(be, nil)
		wantKeys := []ConstValue{{Kind: ConstString, Str: "provider"}, {Kind: ConstString, Str: "model_name"}, {Kind: ConstString, Str: "temperature"}}
		if len(got.Keys) != len(wantKeys) {
			t.Fatalf("Keys length = %d, want %d", len(got.Keys), len(wantKeys))
		}
		for idx, k := range wantKeys {
			if !constValueEqual(got.Keys[idx], k) {
				t.Errorf("Keys[%d] = %#v, want %#v", idx, got.Keys[idx], k)
			}
		}
	})
}

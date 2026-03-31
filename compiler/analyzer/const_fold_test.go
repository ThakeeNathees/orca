package analyzer

import (
	"reflect"
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
			expr: &ast.IntegerLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.INT, Literal: "42"}), Value: 42},
			want: ConstValue{Kind: ConstInt, Int: 42},
		},
		{
			name: "float",
			expr: &ast.FloatLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.FLOAT, Literal: "0.5"}), Value: 0.5},
			want: ConstValue{Kind: ConstFloat, Float: 0.5},
		},
		{
			name: "bool true",
			expr: &ast.BooleanLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.TRUE, Literal: "true"}), Value: true},
			want: ConstValue{Kind: ConstBool, Bool: true},
		},
		{
			name: "null",
			expr: &ast.NullLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.NULL, Literal: "null"})},
			want: ConstValue{Kind: ConstNull},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diags := ConstFold(tt.expr, AnalyzedProgram{})
			if !reflect.DeepEqual(got, tt.want) {
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
	i := func(n int64) *ast.IntegerLiteral {
		return &ast.IntegerLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.INT, Literal: "0"}), Value: n}
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
			want: ConstValue{Kind: ConstList, List: []ConstValue{{Kind: ConstInt, Int: 1}, {Kind: ConstInt, Int: 2}}},
		},
		{
			name: "list with ref fails",
			expr: &ast.ListLiteral{Elements: []ast.Expression{i(1), id("x")}},
			// Unresolved identifiers fold to ConstUnknown per element; the list is still ConstList with Partial set.
			want: ConstValue{Kind: ConstList, List: []ConstValue{{Kind: ConstInt, Int: 1}, {Kind: ConstUnknown}}, Partial: true},
		},
		{
			name: "empty map",
			expr: &ast.MapLiteral{Entries: []ast.MapEntry{}},
			want: ConstValue{Kind: ConstMap, KeyValue: map[string]ConstValue{}},
		},
		{
			name: "map string keys",
			expr: &ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: str("a"), Value: i(1)},
				{Key: str("b"), Value: str("z")},
			}},
			want: ConstValue{Kind: ConstMap, KeyValue: map[string]ConstValue{
				"a": {Kind: ConstInt, Int: 1},
				"b": {Kind: ConstString, Str: "z"},
			}},
		},
		{
			name: "map identifier keys",
			expr: &ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: id("k"), Value: i(7)},
			}},
			// Identifier keys are not ConstString; storage uses keyValue.Str (empty for non-string kinds).
			want: ConstValue{Kind: ConstMap, KeyValue: map[string]ConstValue{"": {Kind: ConstInt, Int: 7}}, Partial: true},
		},
		{
			name: "map int key",
			expr: &ast.MapLiteral{Entries: []ast.MapEntry{
				{Key: i(10), Value: str("ten")},
			}},
			// Integer keys use the same map path; key string is only filled for ConstString keys.
			want: ConstValue{Kind: ConstMap, KeyValue: map[string]ConstValue{"": {Kind: ConstString, Str: "ten"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ConstFold(tt.expr, AnalyzedProgram{})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConstFold() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestConstFoldMapNonStringKeyDiagnostic(t *testing.T) {
	boolKey := &ast.BooleanLiteral{
		BaseNode: ast.NewTerminal(token.Token{Type: token.TRUE, Literal: "true", Line: 5, Column: 3}),
		Value:    true,
	}
	str := &ast.StringLiteral{
		BaseNode: ast.NewTerminal(token.Token{Type: token.STRING, Literal: `"v"`}),
		Value:    "v",
	}
	expr := &ast.MapLiteral{Entries: []ast.MapEntry{
		{Key: boolKey, Value: str},
	}}
	got, diags := ConstFold(expr, AnalyzedProgram{})
	// Non-string keys still produce ConstMap; the key is stored under keyValue.Str (empty for bool).
	if got.Kind != ConstMap || !reflect.DeepEqual(got.KeyValue, map[string]ConstValue{"": {Kind: ConstString, Str: "v"}}) {
		t.Errorf("expected ConstMap with empty-string key, got %#v", got)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Position.Line != 5 || diags[0].Position.Column != 3 {
		t.Errorf("diagnostic position = %d:%d, want 5:3", diags[0].Position.Line, diags[0].Position.Column)
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
				Kind:        token.BlockModel,
				Expressions: nil,
				Assignments: []*ast.Assignment{
					{Name: "provider", Value: str("openai")},
				},
			},
			want: ConstValue{Kind: ConstBlock, KeyValue: map[string]ConstValue{
				"provider": {Kind: ConstString, Str: "openai"},
			}},
		},
		{
			name: "workflow edges not constant",
			be: &ast.BlockExpression{
				Kind:        token.BlockWorkflow,
				Expressions: []ast.Expression{idExpr("a")},
				Assignments: []*ast.Assignment{{Name: "x", Value: str("y")}},
			},
			// Any non-empty Expressions marks the block as non-constant shape; result is ConstBlock with Partial.
			want: ConstValue{Kind: ConstBlock, Partial: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ConstFold(tt.be, AnalyzedProgram{})
			if !reflect.DeepEqual(got, tt.want) {
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
	return &ast.Subscription{Object: obj, Index: index}
}

// TestConstFoldMemberAccess covers foldMemberAccess: ConstBlock field lookup,
// missing members, non-projectable objects, nested member access on constant blocks,
// and member access after foldIdentifier resolves a block ref.
func TestConstFoldMemberAccess(t *testing.T) {
	str := func(s string) *ast.StringLiteral {
		return &ast.StringLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.STRING, Literal: `"x"`}), Value: s}
	}
	i := func(n int64) *ast.IntegerLiteral {
		return &ast.IntegerLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.INT, Literal: "0"}), Value: n}
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
				Kind:        token.BlockModel,
				Expressions: nil,
				Assignments: []*ast.Assignment{
					{Name: "provider", Value: str("acme")},
				},
			}, "provider"),
			want: ConstValue{Kind: ConstString, Str: "acme"},
		},
		{
			name: "block_expression_unknown_field",
			expr: memberAccess(&ast.BlockExpression{
				Kind:        token.BlockModel,
				Assignments: []*ast.Assignment{{Name: "x", Value: i(1)}},
			}, "nope"),
			want:         ConstValue{Kind: ConstUnknown},
			expDiagCodes: []string{diagnostic.CodeUnknownMember},
		},
		{
			name: "nested_block_member_access",
			expr: memberAccess(memberAccess(&ast.BlockExpression{
				Kind: token.BlockModel,
				Assignments: []*ast.Assignment{
					{Name: "inner", Value: &ast.BlockExpression{
						Kind:        token.BlockModel,
						Assignments: []*ast.Assignment{{Name: "temperature", Value: i(2)}},
					}},
				},
			}, "inner"), "temperature"),
			want: ConstValue{Kind: ConstInt, Int: 2},
		},
		{
			name: "string_object_not_folded_member",
			expr: memberAccess(str("hello"), "len"),
			want: ConstValue{Kind: ConstUnknown},
		},
		{
			name: "null_object_not_folded_member",
			expr: memberAccess(&ast.NullLiteral{
				BaseNode: ast.NewTerminal(token.Token{Type: token.NULL, Literal: "null"}),
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
				st.Define("gpt", types.TypeOf(token.BlockModel), token.Token{Type: token.IDENT, Literal: "gpt"})
				return st
			}(),
			program: &ast.Program{
				Statements: []ast.Statement{
					&ast.BlockStatement{
						Name: "gpt",
						Assignments: []*ast.Assignment{
							{Name: "provider", Value: str("openai")},
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
			got, diags := ConstFold(tt.expr, AnalyzedProgram{Ast: tt.program, SymbolTable: tt.symbols})
			if !reflect.DeepEqual(got, tt.want) {
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
	i := func(n int64) *ast.IntegerLiteral {
		return &ast.IntegerLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.INT, Literal: "0"}), Value: n}
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
			want: ConstValue{Kind: ConstInt, Int: 20},
		},
		{
			name: "list_literal_index_first",
			expr: subExpr(&ast.ListLiteral{Elements: []ast.Expression{i(7)}}, i(0)),
			want: ConstValue{Kind: ConstInt, Int: 7},
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
				Kind:        token.BlockModel,
				Assignments: []*ast.Assignment{{Name: "x", Value: i(1)}},
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
			got, diags := ConstFold(tt.expr, AnalyzedProgram{Ast: tt.program, SymbolTable: tt.symbols})
			if !reflect.DeepEqual(got, tt.want) {
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
	i := func(n int64) *ast.IntegerLiteral {
		return &ast.IntegerLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.INT, Literal: "0"}), Value: n}
	}
	f := func(x float64) *ast.FloatLiteral {
		return &ast.FloatLiteral{BaseNode: ast.NewTerminal(token.Token{Type: token.FLOAT, Literal: "0"}), Value: x}
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
		{name: "int add", expr: bin(token.PLUS, i(2), i(3)), want: ConstValue{Kind: ConstInt, Int: 5}},
		{name: "int sub", expr: bin(token.MINUS, i(10), i(4)), want: ConstValue{Kind: ConstInt, Int: 6}},
		{name: "int mul", expr: bin(token.STAR, i(6), i(7)), want: ConstValue{Kind: ConstInt, Int: 42}},
		{name: "int div", expr: bin(token.SLASH, i(7), i(2)), want: ConstValue{Kind: ConstInt, Int: 3}},
		{name: "int div by zero", expr: bin(token.SLASH, i(1), i(0)), want: ConstValue{Kind: ConstUnknown}},
		{name: "float add", expr: bin(token.PLUS, f(1.5), f(2.5)), want: ConstValue{Kind: ConstFloat, Float: 4}},
		{name: "mixed int float", expr: bin(token.PLUS, i(1), f(2.5)), want: ConstValue{Kind: ConstFloat, Float: 3.5}},
		{name: "string concat", expr: bin(token.PLUS, s("a"), s("b")), want: ConstValue{Kind: ConstString, Str: "ab"}},
		{name: "arrow not folded", expr: bin(token.ARROW, idExpr("a"), idExpr("b")), want: ConstValue{Kind: ConstUnknown}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ConstFold(tt.expr, AnalyzedProgram{})
			if !reflect.DeepEqual(got, tt.want) {
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
			name:    "symbol_not_in_table",
			id:      "missing",
			symbols: types.NewSymbolTable(),
			program: nil,
			want:    ConstValue{Kind: ConstUnknown},
		},
		{
			name: "non_blockref_symbol_yields_unknown",
			id:   "xs",
			symbols: func() *types.SymbolTable {
				st := types.NewSymbolTable()
				st.Define("xs", types.NewListType(types.Int()), token.Token{Type: token.IDENT, Literal: "xs"})
				return st
			}(),
			program: nil,
			want:    ConstValue{Kind: ConstUnknown},
		},
		{
			name: "blockref_nil_program",
			id:   "m",
			symbols: func() *types.SymbolTable {
				st := types.NewSymbolTable()
				st.Define("m", types.TypeOf(token.BlockModel), token.Token{Type: token.IDENT, Literal: "m"})
				return st
			}(),
			program: nil,
			want:    ConstValue{Kind: ConstUnknown},
		},
		{
			name: "blockref_no_matching_block",
			id:   "m",
			symbols: func() *types.SymbolTable {
				st := types.NewSymbolTable()
				st.Define("m", types.TypeOf(token.BlockModel), token.Token{Type: token.IDENT, Literal: "m"})
				return st
			}(),
			program: &ast.Program{},
			want:    ConstValue{Kind: ConstUnknown},
		},
		{
			name: "blockref_wrong_block_name_in_program",
			id:   "wanted",
			symbols: func() *types.SymbolTable {
				st := types.NewSymbolTable()
				st.Define("wanted", types.TypeOf(token.BlockModel), token.Token{Type: token.IDENT, Literal: "wanted"})
				return st
			}(),
			program: &ast.Program{
				Statements: []ast.Statement{
					&ast.BlockStatement{Name: "other", Assignments: []*ast.Assignment{{Name: "x", Value: str("y")}}},
				},
			},
			want: ConstValue{Kind: ConstUnknown},
		},
		{
			name: "blockref_folds_matching_block",
			id:   "gpt",
			symbols: func() *types.SymbolTable {
				st := types.NewSymbolTable()
				st.Define("gpt", types.TypeOf(token.BlockModel), token.Token{Type: token.IDENT, Literal: "gpt"})
				return st
			}(),
			program: &ast.Program{
				Statements: []ast.Statement{
					&ast.BlockStatement{
						Name: "gpt",
						Assignments: []*ast.Assignment{
							{Name: "provider", Value: str("openai")},
							{Name: "temperature", Value: &ast.FloatLiteral{
								BaseNode: ast.NewTerminal(token.Token{Type: token.FLOAT, Literal: "0.5"}),
								Value:    0.5,
							}},
						},
					},
				},
			},
			want: ConstValue{
				Kind: ConstBlock,
				KeyValue: map[string]ConstValue{
					"provider":    {Kind: ConstString, Str: "openai"},
					"temperature": {Kind: ConstFloat, Float: 0.5},
				},
			},
		},
		{
			name: "blockref_workflow_edges_partial",
			id:   "wf",
			symbols: func() *types.SymbolTable {
				st := types.NewSymbolTable()
				st.Define("wf", types.TypeOf(token.BlockWorkflow), token.Token{Type: token.IDENT, Literal: "wf"})
				return st
			}(),
			program: &ast.Program{
				Statements: []ast.Statement{
					&ast.BlockStatement{
						Name:        "wf",
						Expressions: []ast.Expression{idExpr("a")},
						Assignments: []*ast.Assignment{
							{Name: "x", Value: str("y")},
						},
					},
				},
			},
			want: ConstValue{Kind: ConstBlock, Partial: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diags := ConstFold(idExpr(tt.id), AnalyzedProgram{Ast: tt.program, SymbolTable: tt.symbols})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConstFold() = %#v, want %#v", got, tt.want)
			}
			if len(diags) != 0 {
				t.Errorf("unexpected diagnostics: %#v", diags)
			}
		})
	}
}

// TestConstFoldLetBoundProviderMember verifies let-bound names fold from their initializer
// when no top-level block shadows the name, so member access (defaults.provider) resolves
// for codegen const folding.
func TestConstFoldLetBoundProviderMember(t *testing.T) {
	input := `let {
  defaults = model {
    provider = "openai"
    model_name = "template"
  }
}
model gpt {
  provider = defaults.provider
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
	v, diags := ConstFold(expr, ap)
	if len(diags) != 0 {
		t.Errorf("unexpected diags: %v", diags)
	}
	want := ConstValue{Kind: ConstString, Str: "openai"}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("ConstFold(provider) = %#v, want %#v", v, want)
	}
}

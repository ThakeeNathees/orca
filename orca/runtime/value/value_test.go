package value

import (
	"testing"

	"github.com/thakee/orca/orca/compiler/analyzer"
	"github.com/thakee/orca/orca/compiler/ast"
	"github.com/thakee/orca/orca/compiler/types"
)

func TestNewNil(t *testing.T) {
	v := NewNil()
	if v.Kind() != KindNil {
		t.Fatalf("Kind() = %v, want KindNil", v.Kind())
	}
	if _, ok := v.String(); ok {
		t.Fatal("String() on nil should fail")
	}
}

func TestNewScalarsAndAccessors(t *testing.T) {
	tests := []struct {
		name string
		v    Value
		want struct {
			str    string
			num    float64
			b      bool
			strOK  bool
			numOK  bool
			boolOK bool
		}
	}{
		{
			name: "string",
			v:    NewString("hello"),
			want: struct {
				str    string
				num    float64
				b      bool
				strOK  bool
				numOK  bool
				boolOK bool
			}{"hello", 0, false, true, false, false},
		},
		{
			name: "number",
			v:    NewNumber(3.5),
			want: struct {
				str    string
				num    float64
				b      bool
				strOK  bool
				numOK  bool
				boolOK bool
			}{"", 3.5, false, false, true, false},
		},
		{
			name: "bool_true",
			v:    NewBool(true),
			want: struct {
				str    string
				num    float64
				b      bool
				strOK  bool
				numOK  bool
				boolOK bool
			}{"", 0, true, false, false, true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, sOK := tt.v.String()
			n, nOK := tt.v.Number()
			b, bOK := tt.v.Bool()
			if s != tt.want.str || sOK != tt.want.strOK {
				t.Errorf("String() = (%q,%v), want (%q,%v)", s, sOK, tt.want.str, tt.want.strOK)
			}
			if n != tt.want.num || nOK != tt.want.numOK {
				t.Errorf("Number() = (%v,%v), want (%v,%v)", n, nOK, tt.want.num, tt.want.numOK)
			}
			if b != tt.want.b || bOK != tt.want.boolOK {
				t.Errorf("Bool() = (%v,%v), want (%v,%v)", b, bOK, tt.want.b, tt.want.boolOK)
			}
		})
	}
}

func TestWrongKindAccessors(t *testing.T) {
	n := NewNumber(1)
	if _, ok := n.String(); ok {
		t.Error("Number.String() should be false")
	}
	if _, ok := n.List(); ok {
		t.Error("Number.List() should be false")
	}
	if _, _, ok := n.KeysValues(); ok {
		t.Error("Number.KeysValues() should be false")
	}
	if _, ok := n.BlockKind(); ok {
		t.Error("Number.BlockKind() should be false")
	}
	if _, _, ok := n.LambdaParts(); ok {
		t.Error("Number.LambdaParts() should be false")
	}
}

func TestNewListEmptyAndCopy(t *testing.T) {
	v := NewList()
	if v.Kind() != KindList {
		t.Fatalf("Kind = %v", v.Kind())
	}
	elems, ok := v.List()
	if !ok || len(elems) != 0 {
		t.Fatalf("List() = %v ok=%v", elems, ok)
	}
	a := NewString("x")
	v2 := NewList(a)
	got, ok := v2.List()
	if !ok || len(got) != 1 || !equalValue(got[0], a) {
		t.Fatalf("list content wrong: %#v", got)
	}
	got = append(got, NewNumber(2))
	got2, _ := v2.List()
	if len(got2) != 1 {
		t.Error("mutating returned slice must not affect stored list")
	}
}

func TestAppend(t *testing.T) {
	tests := []struct {
		name    string
		v       Value
		elem    Value
		wantOk  bool
		wantLen int
	}{
		{name: "empty_append", v: NewList(), elem: NewNumber(1), wantOk: true, wantLen: 1},
		{name: "two_elems", v: NewList(NewBool(false)), elem: NewBool(true), wantOk: true, wantLen: 2},
		{name: "not_list", v: NewString("no"), elem: NewNil(), wantOk: false, wantLen: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, ok := tt.v.Append(tt.elem)
			if ok != tt.wantOk {
				t.Fatalf("Append ok = %v, want %v", ok, tt.wantOk)
			}
			if !tt.wantOk {
				return
			}
			list, ok := out.List()
			if !ok || len(list) != tt.wantLen {
				t.Fatalf("result List len = %d ok=%v, want len %d", len(list), ok, tt.wantLen)
			}
		})
	}
}

func TestNewMapAndBlock(t *testing.T) {
	m := NewMap()
	if m.Kind() != KindMap {
		t.Fatal(m.Kind())
	}
	k, v, ok := m.KeysValues()
	if !ok || len(k) != 0 || len(v) != 0 {
		t.Fatal("empty map keys/vals")
	}
	if _, ok := m.BlockKind(); ok {
		t.Fatal("map should not have BlockKind")
	}

	b := NewBlock("model")
	if b.Kind() != KindBlock {
		t.Fatal(b.Kind())
	}
	bk, ok := b.BlockKind()
	if !ok || bk != "model" {
		t.Fatalf("BlockKind = %q ok=%v", bk, ok)
	}
}

func TestMapWithAppendAndReplace(t *testing.T) {
	k1 := NewString("a")
	k2 := NewString("b")
	m := NewMap()
	m, ok := m.MapWith(k1, NewNumber(1))
	if !ok {
		t.Fatal("first MapWith")
	}
	m, ok = m.MapWith(k2, NewNumber(2))
	if !ok {
		t.Fatal("second MapWith append")
	}
	keys, vals, ok := m.KeysValues()
	if !ok || len(keys) != 2 || len(vals) != 2 {
		t.Fatalf("keys %#v vals %#v", keys, vals)
	}
	m2, ok := m.MapWith(NewString("a"), NewNumber(99))
	if !ok {
		t.Fatal("replace")
	}
	keys2, vals2, _ := m2.KeysValues()
	n, _ := vals2[0].Number()
	if n != 99 {
		t.Fatalf("replace failed, first val = %v", vals2[0])
	}
	// original m unchanged length... actually m is replaced by m2; old m still has old vals
	_, valsOld, _ := m.KeysValues()
	nOld, _ := valsOld[0].Number()
	if nOld != 1 {
		t.Fatalf("old map should still have 1 for key a, got %v", nOld)
	}
	if len(keys2) != 2 {
		t.Fatal("replace should not grow map")
	}
}

func TestMapWithWrongReceiver(t *testing.T) {
	_, ok := NewString("x").MapWith(NewString("k"), NewNil())
	if ok {
		t.Fatal("MapWith on string should fail")
	}
}

func TestBlockMapWithPreservesKind(t *testing.T) {
	b := NewBlock("agent")
	b, ok := b.MapWith(NewString("name"), NewString("x"))
	if !ok || b.Kind() != KindBlock {
		t.Fatal(b)
	}
	bk, _ := b.BlockKind()
	if bk != "agent" {
		t.Fatal(bk)
	}
}

func TestNewLambda(t *testing.T) {
	lam := &ast.Lambda{}
	st := types.NewSymbolTable()
	v := NewLambda(lam, &st)
	l, env, ok := v.LambdaParts()
	if !ok || l != lam || env != &st {
		t.Fatalf("LambdaParts = %p %p ok=%v", l, env, ok)
	}
	v2 := NewLambda(lam, nil)
	_, env2, ok := v2.LambdaParts()
	if !ok || env2 != nil {
		t.Fatal(env2)
	}
}

func TestValueFromConst(t *testing.T) {
	st := types.NewSymbolTable()
	lam := &ast.Lambda{}

	tests := []struct {
		name   string
		cv     analyzer.ConstValue
		sym    *types.SymbolTable
		wantOk bool
		check  func(t *testing.T, v Value)
	}{
		{
			name:   "string",
			cv:     analyzer.ConstValue{Kind: analyzer.ConstString, Str: "hi"},
			wantOk: true,
			check: func(t *testing.T, v Value) {
				s, ok := v.String()
				if !ok || s != "hi" || v.Kind() != KindString {
					t.Fatal(v)
				}
			},
		},
		{
			name:   "number",
			cv:     analyzer.ConstValue{Kind: analyzer.ConstNumber, Number: -2.25},
			wantOk: true,
			check: func(t *testing.T, v Value) {
				n, ok := v.Number()
				if !ok || n != -2.25 {
					t.Fatal(v)
				}
			},
		},
		{
			name:   "bool",
			cv:     analyzer.ConstValue{Kind: analyzer.ConstBool, Bool: true},
			wantOk: true,
			check: func(t *testing.T, v Value) {
				b, ok := v.Bool()
				if !ok || !b {
					t.Fatal(v)
				}
			},
		},
		{
			name:   "null",
			cv:     analyzer.ConstValue{Kind: analyzer.ConstNull},
			wantOk: true,
			check: func(t *testing.T, v Value) {
				if v.Kind() != KindNil {
					t.Fatal(v)
				}
			},
		},
		{
			name:   "unknown",
			cv:     analyzer.ConstValue{Kind: analyzer.ConstUnknown},
			wantOk: false,
		},
		{
			name:   "partial_flag",
			cv:     analyzer.ConstValue{Kind: analyzer.ConstString, Str: "x", Partial: true},
			wantOk: false,
		},
		{
			name: "list_nested",
			cv: analyzer.ConstValue{
				Kind: analyzer.ConstList,
				List: []analyzer.ConstValue{
					{Kind: analyzer.ConstNumber, Number: 1},
					{Kind: analyzer.ConstList, List: []analyzer.ConstValue{
						{Kind: analyzer.ConstString, Str: "inner"},
					}},
				},
			},
			wantOk: true,
			check: func(t *testing.T, v Value) {
				list, ok := v.List()
				if !ok || len(list) != 2 {
					t.Fatal(list)
				}
				inner, ok := list[1].List()
				if !ok || len(inner) != 1 {
					t.Fatal(inner)
				}
			},
		},
		{
			name: "list_child_unknown",
			cv: analyzer.ConstValue{
				Kind: analyzer.ConstList,
				List: []analyzer.ConstValue{
					{Kind: analyzer.ConstUnknown},
				},
			},
			wantOk: false,
		},
		{
			name: "map_pairs",
			cv: analyzer.ConstValue{
				Kind: analyzer.ConstMap,
				Keys: []analyzer.ConstValue{
					{Kind: analyzer.ConstString, Str: "k"},
				},
				Values: []analyzer.ConstValue{
					{Kind: analyzer.ConstBool, Bool: false},
				},
			},
			wantOk: true,
			check: func(t *testing.T, v Value) {
				if v.Kind() != KindMap {
					t.Fatal(v.Kind())
				}
				kk, vv, ok := v.KeysValues()
				if !ok || len(kk) != 1 {
					t.Fatal()
				}
				sk, _ := kk[0].String()
				if sk != "k" {
					t.Fatal(sk)
				}
				b, _ := vv[0].Bool()
				if b {
					t.Fatal("expected false")
				}
			},
		},
		{
			name: "block_with_kind",
			cv: analyzer.ConstValue{
				Kind:      analyzer.ConstBlock,
				BlockKind: "workflow",
				Keys: []analyzer.ConstValue{
					{Kind: analyzer.ConstString, Str: "nodes"},
				},
				Values: []analyzer.ConstValue{
					{Kind: analyzer.ConstString, Str: "n1"},
				},
			},
			wantOk: true,
			check: func(t *testing.T, v Value) {
				if v.Kind() != KindBlock {
					t.Fatal(v.Kind())
				}
				bk, ok := v.BlockKind()
				if !ok || bk != "workflow" {
					t.Fatal(bk)
				}
			},
		},
		{
			name: "lambda_nil_ast",
			cv: analyzer.ConstValue{
				Kind:   analyzer.ConstLambda,
				Lambda: nil,
			},
			sym:    &st,
			wantOk: false,
		},
		{
			name: "lambda_ok",
			cv: analyzer.ConstValue{
				Kind:   analyzer.ConstLambda,
				Lambda: lam,
			},
			sym:    &st,
			wantOk: true,
			check: func(t *testing.T, v Value) {
				l, env, ok := v.LambdaParts()
				if !ok || l != lam || env != &st {
					t.Fatal()
				}
			},
		},
		{
			name:   "lambda_symtab_ignored_for_scalar",
			cv:     analyzer.ConstValue{Kind: analyzer.ConstString, Str: "z"},
			sym:    &st,
			wantOk: true,
			check: func(t *testing.T, v Value) {
				s, _ := v.String()
				if s != "z" {
					t.Fatal(s)
				}
			},
		},
		{
			name: "mismatched_map_lengths",
			cv: analyzer.ConstValue{
				Kind:   analyzer.ConstMap,
				Keys:   []analyzer.ConstValue{{Kind: analyzer.ConstString, Str: "a"}},
				Values: []analyzer.ConstValue{},
			},
			wantOk: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sym := tt.sym
			got, ok := ValueFromConst(tt.cv, sym)
			if ok != tt.wantOk {
				t.Fatalf("ValueFromConst ok = %v, want %v (got %#v)", ok, tt.wantOk, got)
			}
			if tt.wantOk && tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestValueFromConstMapChildUnknown(t *testing.T) {
	cv := analyzer.ConstValue{
		Kind: analyzer.ConstMap,
		Keys: []analyzer.ConstValue{{Kind: analyzer.ConstString, Str: "k"}},
		Values: []analyzer.ConstValue{
			{Kind: analyzer.ConstUnknown},
		},
	}
	_, ok := ValueFromConst(cv, nil)
	if ok {
		t.Fatal("unknown map value should fail conversion")
	}
}

func TestMerge(t *testing.T) {
	mk := func(k, v string) Value {
		m := NewMap()
		m, _ = m.MapWith(NewString(k), NewString(v))
		return m
	}

	tests := []struct {
		name  string
		a     Value
		b     Value
		check func(t *testing.T, got Value)
	}{
		{
			name: "nil_left_returns_b",
			a:    NewNil(),
			b:    NewNumber(42),
			check: func(t *testing.T, got Value) {
				n, ok := got.Number()
				if !ok || n != 42 {
					t.Fatal(got)
				}
			},
		},
		{
			name: "nil_right_returns_a",
			a:    NewString("keep"),
			b:    NewNil(),
			check: func(t *testing.T, got Value) {
				s, ok := got.String()
				if !ok || s != "keep" {
					t.Fatal(got)
				}
			},
		},
		{
			name: "both_nil",
			a:    NewNil(),
			b:    NewNil(),
			check: func(t *testing.T, got Value) {
				if got.Kind() != KindNil {
					t.Fatal(got)
				}
			},
		},
		{
			name: "lists_replace_right_wins",
			a:    NewList(NewNumber(1), NewNumber(2)),
			b:    NewList(NewNumber(3)),
			check: func(t *testing.T, got Value) {
				list, ok := got.List()
				if !ok || len(list) != 1 {
					t.Fatal(list)
				}
				n, _ := list[0].Number()
				if n != 3 {
					t.Fatal(list)
				}
			},
		},
		{
			name: "scalar_replaced_by_right",
			a:    NewNumber(1),
			b:    NewNumber(2),
			check: func(t *testing.T, got Value) {
				n, _ := got.Number()
				if n != 2 {
					t.Fatal(n)
				}
			},
		},
		{
			name: "list_and_scalar_kind_mismatch_right_wins",
			a:    NewList(NewBool(true)),
			b:    NewString("x"),
			check: func(t *testing.T, got Value) {
				s, _ := got.String()
				if s != "x" {
					t.Fatal(got)
				}
			},
		},
		{
			name: "map_merge_new_key",
			a:    mk("a", "1"),
			b:    mk("b", "2"),
			check: func(t *testing.T, got Value) {
				k, vals, ok := got.KeysValues()
				if !ok || len(k) != 2 || len(vals) != 2 {
					t.Fatalf("len keys=%d vals=%d", len(k), len(vals))
				}
			},
		},
		{
			name: "map_merge_shared_key_nested_list_replace",
			a: func() Value {
				m := NewMap()
				m, _ = m.MapWith(NewString("k"), NewList(NewNumber(1)))
				return m
			}(),
			b: func() Value {
				m := NewMap()
				m, _ = m.MapWith(NewString("k"), NewList(NewNumber(2), NewNumber(3)))
				return m
			}(),
			check: func(t *testing.T, got Value) {
				_, vals, _ := got.KeysValues()
				list, _ := vals[0].List()
				if len(list) != 2 {
					t.Fatal(len(list))
				}
				n0, _ := list[0].Number()
				n1, _ := list[1].Number()
				if n0 != 2 || n1 != 3 {
					t.Fatal(list)
				}
			},
		},
		{
			name: "map_merge_shared_key_scalar_replace",
			a: func() Value {
				m := NewMap()
				m, _ = m.MapWith(NewString("x"), NewNumber(1))
				return m
			}(),
			b: func() Value {
				m := NewMap()
				m, _ = m.MapWith(NewString("x"), NewNumber(99))
				return m
			}(),
			check: func(t *testing.T, got Value) {
				_, vals, _ := got.KeysValues()
				n, _ := vals[0].Number()
				if n != 99 {
					t.Fatal(n)
				}
			},
		},
		{
			name: "map_vs_block_right_wins",
			a:    mk("k", "v"),
			b:    NewBlock("model"),
			check: func(t *testing.T, got Value) {
				if got.Kind() != KindBlock {
					t.Fatal(got.Kind())
				}
			},
		},
		{
			name: "block_same_kind_merge",
			a: func() Value {
				b := NewBlock("agent")
				b, _ = b.MapWith(NewString("n"), NewNumber(1))
				return b
			}(),
			b: func() Value {
				b := NewBlock("agent")
				b, _ = b.MapWith(NewString("n"), NewNumber(2))
				return b
			}(),
			check: func(t *testing.T, got Value) {
				bk, ok := got.BlockKind()
				if !ok || bk != "agent" {
					t.Fatal(bk)
				}
				_, vals, _ := got.KeysValues()
				n, _ := vals[0].Number()
				if n != 2 {
					t.Fatal(n)
				}
			},
		},
		{
			name: "block_different_kind_right_wins",
			a: func() Value {
				b := NewBlock("agent")
				b, _ = b.MapWith(NewString("x"), NewNumber(1))
				return b
			}(),
			b: func() Value {
				b := NewBlock("model")
				b, _ = b.MapWith(NewString("y"), NewNumber(2))
				return b
			}(),
			check: func(t *testing.T, got Value) {
				bk, _ := got.BlockKind()
				if bk != "model" {
					t.Fatal(bk)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Merge(tt.a, tt.b)
			tt.check(t, got)
		})
	}
}

func TestValueFromJSON(t *testing.T) {
	tests := []struct {
		name  string
		json  string
		check func(t *testing.T, v Value)
	}{
		{
			name: "nil_whitespace",
			json: "   ",
			check: func(t *testing.T, v Value) {
				if v.Kind() != KindNil {
					t.Fatalf("Kind() = %v, want KindNil", v.Kind())
				}
			},
		},
		{
			name: "number",
			json: "42",
			check: func(t *testing.T, v Value) {
				n, ok := v.Number()
				if !ok || n != 42 {
					t.Fatalf("Number() = (%v,%v), want (42,true)", n, ok)
				}
			},
		},
		{
			name: "string",
			json: `"hi"`,
			check: func(t *testing.T, v Value) {
				s, ok := v.String()
				if !ok || s != "hi" {
					t.Fatalf("String() = (%q,%v), want (%q,true)", s, ok, "hi")
				}
			},
		},
		{
			name: "bool_true",
			json: `true`,
			check: func(t *testing.T, v Value) {
				b, ok := v.Bool()
				if !ok || !b {
					t.Fatalf("Bool() = (%v,%v), want (true,true)", b, ok)
				}
			},
		},
		{
			name: "array",
			json: `[1,2]`,
			check: func(t *testing.T, v Value) {
				list, ok := v.List()
				if !ok || len(list) != 2 {
					t.Fatalf("List len = %v, ok=%v", len(list), ok)
				}
				n0, _ := list[0].Number()
				n1, _ := list[1].Number()
				if n0 != 1 || n1 != 2 {
					t.Fatalf("elements %v %v", n0, n1)
				}
			},
		},
		{
			name: "object",
			json: `{"a":1}`,
			check: func(t *testing.T, v Value) {
				if v.Kind() != KindMap {
					t.Fatalf("Kind() = %v, want KindMap", v.Kind())
				}
				keys, vals, ok := v.KeysValues()
				if !ok || len(keys) != 1 {
					t.Fatalf("KeysValues: ok=%v len=%d", ok, len(keys))
				}
				k, ko := keys[0].String()
				if !ko || k != "a" {
					t.Fatalf("key = %q", k)
				}
				n, no := vals[0].Number()
				if !no || n != 1 {
					t.Fatalf("a = %v", n)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ValueFromJSON([]byte(tt.json))
			if err != nil {
				t.Fatal(err)
			}
			tt.check(t, v)
		})
	}
}

func TestValueFromJSONErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{name: "extra_value", in: `1 2`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValueFromJSON([]byte(tt.in))
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestIsWebhookNode(t *testing.T) {
	b := NewBlock(types.BlockKindWebhook)
	v := b
	if !v.IsWebhookNode() {
		t.Fatal("webhook block should report IsWebhookNode")
	}
	n := NewNumber(1)
	if (&n).IsWebhookNode() {
		t.Fatal("scalar must not be webhook node")
	}
}

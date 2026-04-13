package langgraph

import (
	"strings"
	"testing"

	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/codegen"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
)

// TestExprToSource verifies conversion of AST expressions to Python source.
func TestExprToSource(t *testing.T) {
	tests := []struct {
		name     string
		expr     ast.Expression
		expected string
	}{
		{
			name:     "string literal",
			expr:     &ast.StringLiteral{Value: "hello"},
			expected: `"hello"`,
		},
		{
			name:     "string with quotes",
			expr:     &ast.StringLiteral{Value: `say "hi"`},
			expected: `"say \"hi\""`,
		},
		{
			name:     "raw string with lang",
			expr:     &ast.StringLiteral{Value: "hello world", Lang: "md"},
			expected: `"hello world"`,
		},
		{
			name:     "raw string multiline",
			expr:     &ast.StringLiteral{Value: "line one\nline two", Lang: "py"},
			expected: "\"line one\\nline two\"",
		},
		{
			name:     "integer literal",
			expr:     &ast.NumberLiteral{Value: 42},
			expected: "42",
		},
		{
			name:     "zero integer",
			expr:     &ast.NumberLiteral{Value: 0},
			expected: "0",
		},
		{
			name:     "negative integer",
			expr:     &ast.NumberLiteral{Value: -1},
			expected: "-1",
		},
		{
			name:     "float literal",
			expr:     &ast.NumberLiteral{Value: 3.14},
			expected: "3.14",
		},
		{
			name:     "float whole number",
			expr:     &ast.NumberLiteral{Value: 1.0},
			expected: "1",
		},
		{
			name:     "float zero",
			expr:     &ast.NumberLiteral{Value: 0.0},
			expected: "0",
		},
		{
			name:     "boolean true",
			expr:     &ast.Identifier{Value: "true"},
			expected: "True",
		},
		{
			name:     "boolean false",
			expr:     &ast.Identifier{Value: "false"},
			expected: "False",
		},
		{
			name:     "null identifier",
			expr:     &ast.Identifier{Value: "null"},
			expected: "None",
		},
		{
			name:     "identifier",
			expr:     &ast.Identifier{Value: "my_var"},
			expected: "my_var",
		},
		{
			name: "member access",
			expr: &ast.MemberAccess{
				Object: &ast.Identifier{Value: "config"},
				Member: "timeout",
			},
			expected: "config.timeout",
		},
		{
			name: "nested member access",
			expr: &ast.MemberAccess{
				Object: &ast.MemberAccess{
					Object: &ast.Identifier{Value: "a"},
					Member: "b",
				},
				Member: "c",
			},
			expected: "a.b.c",
		},
		{
			name: "subscription with integer",
			expr: &ast.Subscription{
				Object:  &ast.Identifier{Value: "items"},
				Indices: []ast.Expression{&ast.NumberLiteral{Value: 0}},
			},
			expected: "items[0]",
		},
		{
			name: "subscription with string key",
			expr: &ast.Subscription{
				Object:  &ast.Identifier{Value: "data"},
				Indices: []ast.Expression{&ast.StringLiteral{Value: "key"}},
			},
			expected: `data["key"]`,
		},
		{
			name:     "empty list",
			expr:     &ast.ListLiteral{Elements: []ast.Expression{}},
			expected: "[]",
		},
		{
			name: "list with elements",
			expr: &ast.ListLiteral{
				Elements: []ast.Expression{
					&ast.NumberLiteral{Value: 1},
					&ast.NumberLiteral{Value: 2},
					&ast.NumberLiteral{Value: 3},
				},
			},
			expected: "[1, 2, 3]",
		},
		{
			name: "list with mixed types",
			expr: &ast.ListLiteral{
				Elements: []ast.Expression{
					&ast.StringLiteral{Value: "a"},
					&ast.NumberLiteral{Value: 1},
					&ast.Identifier{Value: "true"},
				},
			},
			expected: `["a", 1, True]`,
		},
		{
			name:     "empty map",
			expr:     &ast.MapLiteral{Entries: []ast.MapEntry{}},
			expected: "{}",
		},
		{
			name: "map with entries",
			expr: &ast.MapLiteral{
				Entries: []ast.MapEntry{
					{Key: &ast.StringLiteral{Value: "name"}, Value: &ast.StringLiteral{Value: "orca"}},
					{Key: &ast.StringLiteral{Value: "version"}, Value: &ast.NumberLiteral{Value: 1}},
				},
			},
			expected: `{"name": "orca", "version": 1}`,
		},
		{
			name: "binary expression",
			expr: &ast.BinaryExpression{
				Left:     &ast.NumberLiteral{Value: 1},
				Operator: token.Token{Literal: "+"},
				Right:    &ast.NumberLiteral{Value: 2},
			},
			expected: "1 + 2",
		},
		{
			name: "binary expression with strings",
			expr: &ast.BinaryExpression{
				Left:     &ast.StringLiteral{Value: "hello"},
				Operator: token.Token{Literal: "+"},
				Right:    &ast.StringLiteral{Value: " world"},
			},
			expected: `"hello" + " world"`,
		},
		{
			name: "call expression no args",
			expr: &ast.CallExpression{
				Callee:    &ast.Identifier{Value: "foo"},
				Arguments: []ast.Expression{},
			},
			expected: "foo()",
		},
		{
			name: "call expression with args",
			expr: &ast.CallExpression{
				Callee: &ast.Identifier{Value: "max"},
				Arguments: []ast.Expression{
					&ast.NumberLiteral{Value: 1},
					&ast.NumberLiteral{Value: 2},
				},
			},
			expected: "max(1, 2)",
		},
		{
			name: "method call via member access",
			expr: &ast.CallExpression{
				Callee: &ast.MemberAccess{
					Object: &ast.Identifier{Value: "obj"},
					Member: "method",
				},
				Arguments: []ast.Expression{
					&ast.StringLiteral{Value: "arg"},
				},
			},
			expected: `obj.method("arg")`,
		},
		{
			name: "nested list in map",
			expr: &ast.MapLiteral{
				Entries: []ast.MapEntry{
					{
						Key: &ast.StringLiteral{Value: "items"},
						Value: &ast.ListLiteral{
							Elements: []ast.Expression{
								&ast.NumberLiteral{Value: 1},
								&ast.NumberLiteral{Value: 2},
							},
						},
					},
				},
			},
			expected: `{"items": [1, 2]}`,
		},
		{
			name: "lambda with params",
			expr: &ast.Lambda{
				Params: []ast.LambdaParam{
					{Name: &ast.Identifier{Value: "a"}, TypeExpr: &ast.Identifier{Value: "number"}},
					{Name: &ast.Identifier{Value: "b"}, TypeExpr: &ast.Identifier{Value: "number"}},
				},
				Body: &ast.BinaryExpression{
					Left:     &ast.Identifier{Value: "a"},
					Operator: token.Token{Literal: "+"},
					Right:    &ast.Identifier{Value: "b"},
				},
			},
			expected: "lambda a, b: a + b",
		},
		{
			name: "lambda zero params",
			expr: &ast.Lambda{
				Body: &ast.NumberLiteral{Value: 42},
			},
			expected: "lambda: 42",
		},
		{
			name: "block expression with assignments",
			expr: &ast.BlockExpression{BlockBody: ast.BlockBody{
				Kind: types.BlockKindModel,
				Assignments: []*ast.Assignment{
					{Name: "provider", Value: &ast.StringLiteral{Value: "openai"}},
					{Name: "model_name", Value: &ast.StringLiteral{Value: "gpt-4o"}},
				},
			}},
			expected: `_orca__block("model", provider="openai", model_name="gpt-4o", )`,
		},
		{
			name:     "empty block expression",
			expr:     &ast.BlockExpression{BlockBody: ast.BlockBody{Kind: types.BlockKindAgent}},
			expected: `_orca__block("agent", )`,
		},
		{
			name: "block expression field with one annotation",
			expr: &ast.BlockExpression{BlockBody: ast.BlockBody{
				Kind: types.BlockKindSchema,
				Assignments: []*ast.Assignment{
					{
						Name: "region",
						Annotations: []*ast.Annotation{
							{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "AWS region"}}},
						},
						Value: &ast.Identifier{Value: "string"},
					},
				},
			}},
			expected: "_orca__block(\"schema\", region=_orca__with_meta(\n" +
				"    str,\n" +
				"    [\n" +
				"        _orca__meta(\"desc\", \"AWS region\"),\n" +
				"    ],\n" +
				"), )",
		},
		{
			name: "block expression field with multiple annotations",
			expr: &ast.BlockExpression{BlockBody: ast.BlockBody{
				Kind: types.BlockKindSchema,
				Assignments: []*ast.Assignment{
					{
						Name: "region",
						Annotations: []*ast.Annotation{
							{Name: "required"},
							{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "r"}}},
						},
						Value: &ast.Identifier{Value: "string"},
					},
				},
			}},
			expected: "_orca__block(\"schema\", region=_orca__with_meta(\n" +
				"    str,\n" +
				"    [\n" +
				"        _orca__meta(\"required\"),\n" +
				"        _orca__meta(\"desc\", \"r\"),\n" +
				"    ],\n" +
				"), )",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &LangGraphBackend{}
			got := b.exprToSource(tt.expr)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// TestExprToSourceNilExpression verifies a nil Expression maps to Python None.
func TestExprToSourceNilExpression(t *testing.T) {
	b := &LangGraphBackend{}
	if got := b.exprToSource(nil); got != "None" {
		t.Fatalf("exprToSource(nil) = %q, want None", got)
	}
}

// TestExprToSourceExhaustiveKinds runs exprToSource once per known Expression
// concrete type from ast.go so adding a new type without a codegen case tends
// to fail CI (this table must be updated alongside ast.Expression implementers).
func TestExprToSourceExhaustiveKinds(t *testing.T) {
	cases := []struct {
		name string
		expr ast.Expression
	}{
		{"StringLiteral", &ast.StringLiteral{Value: "x"}},
		{"NumberLiteral", &ast.NumberLiteral{Value: 1}},
		{"Identifier_null", &ast.Identifier{Value: "null"}},
		{"Identifier", &ast.Identifier{Value: "id"}},
		{"MemberAccess", &ast.MemberAccess{Object: &ast.Identifier{Value: "a"}, Member: "b"}},
		{"Subscription", &ast.Subscription{Object: &ast.Identifier{Value: "a"}, Indices: []ast.Expression{&ast.NumberLiteral{Value: 0}}}},
		{"ListLiteral", &ast.ListLiteral{Elements: []ast.Expression{&ast.NumberLiteral{Value: 1}}}},
		{"MapLiteral", &ast.MapLiteral{Entries: []ast.MapEntry{{Key: &ast.StringLiteral{Value: "k"}, Value: &ast.NumberLiteral{Value: 1}}}}},
		{"BinaryExpression", &ast.BinaryExpression{Left: &ast.Identifier{Value: "a"}, Operator: token.Token{Literal: "->"}, Right: &ast.Identifier{Value: "b"}}},
		{"CallExpression", &ast.CallExpression{Callee: &ast.Identifier{Value: "f"}, Arguments: []ast.Expression{}}},
		{"BlockExpression", &ast.BlockExpression{BlockBody: ast.BlockBody{Kind: types.BlockKindModel, Assignments: []*ast.Assignment{{Name: "provider", Value: &ast.StringLiteral{Value: "openai"}}}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic: %v", r)
				}
			}()
			b := &LangGraphBackend{}
			if b.exprToSource(tc.expr) == "" {
				t.Fatal("empty result")
			}
		})
	}
}

// TestAnnotationToSource verifies codegen for a single @annotation.
func TestAnnotationToSource(t *testing.T) {
	tests := []struct {
		name string
		ann  *ast.Annotation
		want string
	}{
		{
			name: "no arguments",
			ann:  &ast.Annotation{Name: "sensitive"},
			want: `_orca__meta("sensitive")`,
		},
		{
			name: "one string argument",
			ann: &ast.Annotation{
				Name:      "desc",
				Arguments: []ast.Expression{&ast.StringLiteral{Value: "hello"}},
			},
			want: `_orca__meta("desc", "hello")`,
		},
		{
			name: "expression argument",
			ann: &ast.Annotation{
				Name: "ann1",
				Arguments: []ast.Expression{
					&ast.MemberAccess{Object: &ast.Identifier{Value: "some"}, Member: "expr1"},
				},
			},
			want: `_orca__meta("ann1", some.expr1)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &LangGraphBackend{}
			got := annotationToSource(b, tt.ann)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWrapWithMetaIfNeeded verifies with_meta is omitted when there are no annotations.
func TestWrapWithMetaIfNeeded(t *testing.T) {
	tests := []struct {
		name        string
		inner       string
		anns        []*ast.Annotation
		argIndent   string
		closeIndent string
		want        string
	}{
		{
			name:        "nil annotations",
			inner:       `_orca__foo()`,
			anns:        nil,
			argIndent:   "    ",
			closeIndent: "",
			want:        `_orca__foo()`,
		},
		{
			name:        "empty slice",
			inner:       `_orca__foo()`,
			anns:        []*ast.Annotation{},
			argIndent:   "    ",
			closeIndent: "",
			want:        `_orca__foo()`,
		},
		{
			name:        "single block-level style annotation",
			inner:       `_orca__model(provider="openai",model_name="gpt-4o",)`,
			anns:        []*ast.Annotation{{Name: "sensitive"}},
			argIndent:   "    ",
			closeIndent: "",
			want: "_orca__with_meta(\n" +
				"    _orca__model(provider=\"openai\",model_name=\"gpt-4o\",),\n" +
				"    [\n" +
				"        _orca__meta(\"sensitive\"),\n" +
				"    ],\n" +
				")",
		},
		{
			name:  "multiple annotations list",
			inner: `_orca__bar()`,
			anns: []*ast.Annotation{
				{Name: "a", Arguments: []ast.Expression{&ast.NumberLiteral{Value: 1}}},
				{Name: "b"},
			},
			argIndent:   "    ",
			closeIndent: "",
			want: "_orca__with_meta(\n" +
				"    _orca__bar(),\n" +
				"    [\n" +
				"        _orca__meta(\"a\", 1),\n" +
				"        _orca__meta(\"b\"),\n" +
				"    ],\n" +
				")",
		},
		{
			name:        "deeper arg indent for nested context",
			inner:       `str`,
			anns:        []*ast.Annotation{{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "x"}}}},
			argIndent:   "        ",
			closeIndent: "    ",
			want: "_orca__with_meta(\n" +
				"        str,\n" +
				"        [\n" +
				"            _orca__meta(\"desc\", \"x\"),\n" +
				"        ],\n" +
				"    )",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &LangGraphBackend{}
			got := wrapWithMetaIfNeeded(b, tt.inner, tt.anns, tt.argIndent, tt.closeIndent)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestTopLevelBlockSource verifies block-level annotations wrap the whole _orca__* call.
func TestTopLevelBlockSource(t *testing.T) {
	block := &ast.BlockStatement{
		Annotations: []*ast.Annotation{
			{Name: "sensitive"},
		},
		BlockBody: ast.BlockBody{
			Name: "gpt4",
			Kind: types.BlockKindModel,
			Assignments: []*ast.Assignment{
				{Name: "provider", Value: &ast.StringLiteral{Value: "openai"}},
				{Name: "model_name", Value: &ast.StringLiteral{Value: "gpt-4o"}},
			},
		},
	}
	b := &LangGraphBackend{}
	got := topLevelBlockSource(b, block)
	if !strings.Contains(got, `_orca__with_meta(`) || !strings.Contains(got, `_orca__meta("sensitive")`) {
		t.Fatalf("expected with_meta and sensitive meta, got:\n%s", got)
	}
	if !strings.Contains(got, `_orca__block("model", `) {
		t.Fatalf("expected inner _orca__block(\"model\", ...) call, got:\n%s", got)
	}

	plain := &ast.BlockStatement{
		BlockBody: ast.BlockBody{
			Name: "x",
			Kind: types.BlockKindModel,
			Assignments: []*ast.Assignment{
				{Name: "provider", Value: &ast.StringLiteral{Value: "openai"}},
				{Name: "model_name", Value: &ast.StringLiteral{Value: "gpt-4o"}},
			},
		},
	}
	plainGot := topLevelBlockSource(b, plain)
	if strings.Contains(plainGot, "with_meta") {
		t.Fatalf("expected no with_meta without block annotations, got %q", plainGot)
	}
}

// TestFormatFloat verifies float formatting for Python output.
func TestFormatFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{"zero", 0.0, "0"},
		{"whole number", 1.0, "1"},
		{"decimal", 3.14, "3.14"},
		{"small decimal", 0.001, "0.001"},
		{"large number", 1000000.0, "1000000"},
		{"negative", -2.5, "-2.5"},
		{"negative whole", -1.0, "-1"},
		{"half", 0.5, "0.5"},
		{"very small", 0.0001, "0.0001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFloat(tt.input)
			if got != tt.expected {
				t.Errorf("formatFloat(%v): expected %q, got %q", tt.input, tt.expected, got)
			}
		})
	}
}

// TestConstValToSource covers each ConstKind rendered to Python source.
// Partial/Unknown children that fall back to the AST are covered separately
// in TestExprToSourceCachePath.
func TestConstValToSource(t *testing.T) {
	tests := []struct {
		name string
		in   analyzer.ConstValue
		want string
	}{
		{"string", analyzer.ConstValue{Kind: analyzer.ConstString, Str: "hello"}, `"hello"`},
		{"string with quotes", analyzer.ConstValue{Kind: analyzer.ConstString, Str: `say "hi"`}, `"say \"hi\""`},
		{"number whole", analyzer.ConstValue{Kind: analyzer.ConstNumber, Number: 42}, "42"},
		{"number fractional", analyzer.ConstValue{Kind: analyzer.ConstNumber, Number: 0.5}, "0.5"},
		{"bool true", analyzer.ConstValue{Kind: analyzer.ConstBool, Bool: true}, "True"},
		{"bool false", analyzer.ConstValue{Kind: analyzer.ConstBool, Bool: false}, "False"},
		{"null", analyzer.ConstValue{Kind: analyzer.ConstNull}, "None"},
		{
			name: "list of mixed primitives",
			in: analyzer.ConstValue{Kind: analyzer.ConstList, List: []analyzer.ConstValue{
				{Kind: analyzer.ConstNumber, Number: 1},
				{Kind: analyzer.ConstString, Str: "two"},
				{Kind: analyzer.ConstBool, Bool: true},
			}},
			want: `[1, "two", True]`,
		},
		{
			name: "map preserves source order",
			in: analyzer.ConstValue{
				Kind:   analyzer.ConstMap,
				Keys:   []string{"z", "a"},
				Values: []analyzer.ConstValue{{Kind: analyzer.ConstNumber, Number: 1}, {Kind: analyzer.ConstNumber, Number: 2}},
			},
			want: `{"z": 1, "a": 2}`,
		},
		// Note: ConstBlock is intentionally not tested here — constValToSource
		// routes ConstBlock values back through exprToSource(constVal.Expr) so
		// named block references stay as identifiers instead of being inlined
		// (see TestExprToSourceNamedBlockRefNotInlined).
	}

	b := &LangGraphBackend{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := constValToSource(b, tt.in)
			if got != tt.want {
				t.Errorf("constValToSource() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestExprToSourceCachePath verifies that when ConstFoldCache contains a
// non-Unknown entry for an expression, exprToSource routes it through
// constValToSource, and that a ConstUnknown entry is bypassed (falling back to
// the AST switch). A zero backend with no cache should always use the AST path.
func TestExprToSourceCachePath(t *testing.T) {
	numExpr := &ast.NumberLiteral{
		BaseNode: ast.NewTerminal(token.Token{Type: token.NUMBER, Literal: "1"}),
		Value:    1,
	}

	t.Run("cache hit routes through constValToSource", func(t *testing.T) {
		b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: analyzer.AnalyzedProgram{
			ConstFoldCache: map[ast.Expression]analyzer.ConstValue{
				numExpr: {Kind: analyzer.ConstNumber, Number: 999},
			},
		}}}
		if got := b.exprToSource(numExpr); got != "999" {
			t.Errorf("exprToSource() = %q, want cached %q", got, "999")
		}
	})

	t.Run("unknown cache entry falls through to AST", func(t *testing.T) {
		b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: analyzer.AnalyzedProgram{
			ConstFoldCache: map[ast.Expression]analyzer.ConstValue{
				numExpr: {Kind: analyzer.ConstUnknown, Expr: numExpr},
			},
		}}}
		// The AST path emits the literal value; the cached Unknown must not short-circuit.
		if got := b.exprToSource(numExpr); got != "1" {
			t.Errorf("exprToSource() = %q, want AST path %q", got, "1")
		}
	})

	t.Run("no cache uses AST path", func(t *testing.T) {
		b := &LangGraphBackend{}
		if got := b.exprToSource(numExpr); got != "1" {
			t.Errorf("exprToSource() = %q, want %q", got, "1")
		}
	})
}

// TestExprToSourceNamedBlockRefNotInlined is a regression test: an identifier
// referencing a named top-level block must emit as the Python variable name,
// not get replaced by an inlined _orca__block(...) call sourced from the
// ConstFoldCache. Without this guard the single top-level block definition
// becomes unused and each reference duplicates the block body.
func TestExprToSourceNamedBlockRefNotInlined(t *testing.T) {
	src := `model gpt4 {
  provider = "openai"
  model_name = "gpt-4o"
}
agent writer {
  model = gpt4
  persona = "You write things."
}`
	l := lexer.New(src, "")
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	ap := analyzer.Analyze(program)
	if len(ap.Diagnostics) > 0 {
		t.Fatalf("analyze diagnostics: %v", ap.Diagnostics)
	}

	writer := program.FindBlockWithName("writer")
	if writer == nil {
		t.Fatal("writer block not found")
	}
	modelField, ok := writer.GetFieldExpression("model")
	if !ok {
		t.Fatal("writer.model field missing")
	}

	b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: ap}}
	got := b.exprToSource(modelField)
	if got != "gpt4" {
		t.Errorf("exprToSource(writer.model) = %q, want %q (named block refs must not be inlined)", got, "gpt4")
	}
	if strings.Contains(got, "_orca__block") {
		t.Errorf("named block identifier was inlined: %q", got)
	}
}

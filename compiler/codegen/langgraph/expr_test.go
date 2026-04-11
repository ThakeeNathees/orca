package langgraph

import (
	"strings"
	"testing"

	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/ast"
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
				Object: &ast.Identifier{Value: "items"},
				Indices: []ast.Expression{&ast.NumberLiteral{Value: 0}},
			},
			expected: "items[0]",
		},
		{
			name: "subscription with string key",
			expr: &ast.Subscription{
				Object: &ast.Identifier{Value: "data"},
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
			expected: "lambda : 42",
		},
		{
			name: "block expression with assignments",
			expr: &ast.BlockExpression{BlockBody: ast.BlockBody{
				Kind: analyzer.BlockKindModel,
				Assignments: []*ast.Assignment{
					{Name: "provider", Value: &ast.StringLiteral{Value: "openai"}},
					{Name: "model_name", Value: &ast.StringLiteral{Value: "gpt-4o"}},
				},
			}},
			expected: `__orca_block("model", provider="openai", model_name="gpt-4o", )`,
		},
		{
			name:     "empty block expression",
			expr:     &ast.BlockExpression{BlockBody: ast.BlockBody{Kind: analyzer.BlockKindAgent}},
			expected: `__orca_block("agent", )`,
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
			expected: "__orca_block(\"schema\", region=__orca_with_meta(\n" +
				"    str,\n" +
				"    [\n" +
				"        __orca_meta(\"desc\", \"AWS region\"),\n" +
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
			expected: "__orca_block(\"schema\", region=__orca_with_meta(\n" +
				"    str,\n" +
				"    [\n" +
				"        __orca_meta(\"required\"),\n" +
				"        __orca_meta(\"desc\", \"r\"),\n" +
				"    ],\n" +
				"), )",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exprToSource(tt.expr)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// TestExprToSourceNilExpression verifies a nil Expression maps to Python None.
func TestExprToSourceNilExpression(t *testing.T) {
	if got := exprToSource(nil); got != "None" {
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
		{"BlockExpression", &ast.BlockExpression{BlockBody: ast.BlockBody{Kind: analyzer.BlockKindModel, Assignments: []*ast.Assignment{{Name: "provider", Value: &ast.StringLiteral{Value: "openai"}}}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic: %v", r)
				}
			}()
			if exprToSource(tc.expr) == "" {
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
			want: `__orca_meta("sensitive")`,
		},
		{
			name: "one string argument",
			ann: &ast.Annotation{
				Name:      "desc",
				Arguments: []ast.Expression{&ast.StringLiteral{Value: "hello"}},
			},
			want: `__orca_meta("desc", "hello")`,
		},
		{
			name: "expression argument",
			ann: &ast.Annotation{
				Name: "ann1",
				Arguments: []ast.Expression{
					&ast.MemberAccess{Object: &ast.Identifier{Value: "some"}, Member: "expr1"},
				},
			},
			want: `__orca_meta("ann1", some.expr1)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := annotationToSource(tt.ann)
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
			inner:       `__orca_foo()`,
			anns:        nil,
			argIndent:   "    ",
			closeIndent: "",
			want:        `__orca_foo()`,
		},
		{
			name:        "empty slice",
			inner:       `__orca_foo()`,
			anns:        []*ast.Annotation{},
			argIndent:   "    ",
			closeIndent: "",
			want:        `__orca_foo()`,
		},
		{
			name:        "single block-level style annotation",
			inner:       `__orca_model(provider="openai",model_name="gpt-4o",)`,
			anns:        []*ast.Annotation{{Name: "sensitive"}},
			argIndent:   "    ",
			closeIndent: "",
			want: "__orca_with_meta(\n" +
				"    __orca_model(provider=\"openai\",model_name=\"gpt-4o\",),\n" +
				"    [\n" +
				"        __orca_meta(\"sensitive\"),\n" +
				"    ],\n" +
				")",
		},
		{
			name:  "multiple annotations list",
			inner: `__orca_bar()`,
			anns: []*ast.Annotation{
				{Name: "a", Arguments: []ast.Expression{&ast.NumberLiteral{Value: 1}}},
				{Name: "b"},
			},
			argIndent:   "    ",
			closeIndent: "",
			want: "__orca_with_meta(\n" +
				"    __orca_bar(),\n" +
				"    [\n" +
				"        __orca_meta(\"a\", 1),\n" +
				"        __orca_meta(\"b\"),\n" +
				"    ],\n" +
				")",
		},
		{
			name:        "deeper arg indent for nested context",
			inner:       `str`,
			anns:        []*ast.Annotation{{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "x"}}}},
			argIndent:   "        ",
			closeIndent: "    ",
			want: "__orca_with_meta(\n" +
				"        str,\n" +
				"        [\n" +
				"            __orca_meta(\"desc\", \"x\"),\n" +
				"        ],\n" +
				"    )",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapWithMetaIfNeeded(tt.inner, tt.anns, tt.argIndent, tt.closeIndent)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestTopLevelBlockSource verifies block-level annotations wrap the whole __orca_* call.
func TestTopLevelBlockSource(t *testing.T) {
	block := &ast.BlockStatement{
		Name: "gpt4",
		Annotations: []*ast.Annotation{
			{Name: "sensitive"},
		},
		BlockBody: ast.BlockBody{
			Kind: analyzer.BlockKindModel,
			Assignments: []*ast.Assignment{
				{Name: "provider", Value: &ast.StringLiteral{Value: "openai"}},
				{Name: "model_name", Value: &ast.StringLiteral{Value: "gpt-4o"}},
			},
		},
	}
	got := topLevelBlockSource(block)
	if !strings.Contains(got, `__orca_with_meta(`) || !strings.Contains(got, `__orca_meta("sensitive")`) {
		t.Fatalf("expected with_meta and sensitive meta, got:\n%s", got)
	}
	if !strings.Contains(got, `__orca_block("model", `) {
		t.Fatalf("expected inner __orca_block(\"model\", ...) call, got:\n%s", got)
	}

	plain := &ast.BlockStatement{
		Name: "x",
		BlockBody: ast.BlockBody{
			Kind: analyzer.BlockKindModel,
			Assignments: []*ast.Assignment{
				{Name: "provider", Value: &ast.StringLiteral{Value: "openai"}},
				{Name: "model_name", Value: &ast.StringLiteral{Value: "gpt-4o"}},
			},
		},
	}
	plainGot := topLevelBlockSource(plain)
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

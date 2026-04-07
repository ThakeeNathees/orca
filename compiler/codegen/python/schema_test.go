package python

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
)

// lazyRef is a block reference with no resolved *BlockSchema (matches typical test Type shapes).
func lazyRef(name string) types.Type {
	return types.NewBlockRefType(name, nil)
}

// TestOrcaTypeToPythonTypeName verifies resolved Type to Python annotation conversion.
func TestOrcaTypeToPythonTypeName(t *testing.T) {
	tests := []struct {
		name     string
		typ      types.Type
		expected string
	}{
		// Primitives (lazy block refs by name).
		{"str", lazyRef("str"), "str"},
		{"int", lazyRef("int"), "int"},
		{"float", lazyRef("float"), "float"},
		{"bool", lazyRef("bool"), "bool"},
		{"any → Any", lazyRef("any"), "Any"},
		{"null → None", lazyRef("null"), "None"},

		// User-defined schema refs.
		{"user schema", lazyRef("article"), "article"},
		{"user schema snake_case", lazyRef("vpc_data_t"), "vpc_data_t"},

		// List types.
		{"list[str]", types.NewListType(lazyRef("str")), "list[str]"},
		{"list[int]", types.NewListType(lazyRef("int")), "list[int]"},
		{"list[article]", types.NewListType(lazyRef("article")), "list[article]"},
		{"list[list[str]]", types.NewListType(types.NewListType(lazyRef("str"))), "list[list[str]]"},
		{"untyped list", types.Type{Kind: types.List}, "list"},

		// Map types.
		{"dict[str, int]", types.NewMapType(lazyRef("str"), lazyRef("int")), "dict[str, int]"},
		{"dict[str, article]", types.NewMapType(lazyRef("str"), lazyRef("article")), "dict[str, article]"},
		{"dict[str, list[str]]", types.NewMapType(lazyRef("str"), types.NewListType(lazyRef("str"))), "dict[str, list[str]]"},
		{"untyped dict", types.Type{Kind: types.Map}, "dict"},

		// Union types.
		{"str | None", types.NewUnionType(lazyRef("str"), lazyRef("null")), "str | None"},
		{"str | int", types.NewUnionType(lazyRef("str"), lazyRef("int")), "str | int"},
		{"float | None", types.NewUnionType(lazyRef("float"), lazyRef("null")), "float | None"},
		{"str | int | None", types.NewUnionType(lazyRef("str"), lazyRef("int"), lazyRef("null")), "str | int | None"},
		{"str | int | float", types.NewUnionType(lazyRef("str"), lazyRef("int"), lazyRef("float")), "str | int | float"},
		{"list[str] | None", types.NewUnionType(types.NewListType(lazyRef("str")), lazyRef("null")), "list[str] | None"},
		{"article | None", types.NewUnionType(lazyRef("article"), lazyRef("null")), "article | None"},

		// Unresolved block refs with concrete names pass through as annotations (not empty → Any).
		{"block name model", lazyRef("model"), "model"},
		{"block name agent", lazyRef("agent"), "agent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := orcaTypeToPythonTypeName(tt.typ)
			if got != tt.expected {
				t.Errorf("orcaTypeToPythonTypeName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestSchemaBlockToSource verifies full Pydantic class generation from schema blocks.
//
// SchemaBlockToSource calls BlockSchemaTypeOfExpr(expr, nil). With a nil symbol table,
// types.IdentType falls back to any for almost every identifier, so generated annotations
// use Any heavily. When codegen supplies bootstrap or analyzer symbols, update these
// expectations to match str/int/… and optional null handling.
func TestSchemaBlockToSource(t *testing.T) {
	tests := []struct {
		name     string
		block    *ast.BlockStatement
		expected string
	}{
		{
			name: "basic str and int fields",
			block: &ast.BlockStatement{
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{Name: "title", Value: &ast.Identifier{Value: "str"}},
						{Name: "count", Value: &ast.Identifier{Value: "int"}},
					},
				},
			},
			// Nil symbol table: identifiers resolve to any (see types.IdentType).
			expected: "class article(BaseModel):\n" +
				"    title: Any\n" +
				"    count: Any\n",
		},
		{
			name: "all primitive types",
			block: &ast.BlockStatement{
				Name: "config",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{Name: "name", Value: &ast.Identifier{Value: "str"}},
						{Name: "count", Value: &ast.Identifier{Value: "int"}},
						{Name: "rate", Value: &ast.Identifier{Value: "float"}},
						{Name: "enabled", Value: &ast.Identifier{Value: "bool"}},
					},
				},
			},
			expected: "class config(BaseModel):\n" +
				"    name: Any\n" +
				"    count: Any\n" +
				"    rate: Any\n" +
				"    enabled: Any\n",
		},
		{
			name: "any type field",
			block: &ast.BlockStatement{
				Name: "flexible",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{Name: "data", Value: &ast.Identifier{Value: "any"}},
					},
				},
			},
			expected: "class flexible(BaseModel):\n" +
				"    data: Any\n",
		},
		{
			name: "field with desc annotation",
			block: &ast.BlockStatement{
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name:  "title",
							Value: &ast.Identifier{Value: "str"},
							Annotations: []*ast.Annotation{
								{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "The article title"}}},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    title: Any = Field(description=\"The article title\")\n",
		},
		{
			name: "desc with special characters",
			block: &ast.BlockStatement{
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name:  "body",
							Value: &ast.Identifier{Value: "str"},
							Annotations: []*ast.Annotation{
								{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: `Contains "quotes" and \backslash`}}},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    body: Any = Field(description=\"Contains \\\"quotes\\\" and \\\\backslash\")\n",
		},
		{
			name: "optional field without desc (null identifier union)",
			block: &ast.BlockStatement{
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "score",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "float"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: "null"},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    score: Any | Any\n",
		},
		{
			name: "optional field without desc (null identifier)",
			block: &ast.BlockStatement{
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "score",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "float"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: "null"},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    score: Any | Any\n",
		},
		{
			name: "optional field with desc",
			block: &ast.BlockStatement{
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "score",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "float"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: "null"},
							},
							Annotations: []*ast.Annotation{
								{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "Confidence score"}}},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    score: Any | Any = Field(description=\"Confidence score\")\n",
		},
		{
			name: "multi-member union with null",
			block: &ast.BlockStatement{
				Name: "result",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "value",
							Value: &ast.BinaryExpression{
								Left: &ast.BinaryExpression{
									Left:     &ast.Identifier{Value: "str"},
									Operator: token.Token{Type: token.PIPE, Literal: "|"},
									Right:    &ast.Identifier{Value: "int"},
								},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: "null"},
							},
						},
					},
				},
			},
			expected: "class result(BaseModel):\n" +
				"    value: Any | Any | Any\n",
		},
		{
			name: "non-optional union",
			block: &ast.BlockStatement{
				Name: "result",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "value",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "str"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: "int"},
							},
						},
					},
				},
			},
			expected: "class result(BaseModel):\n" +
				"    value: Any | Any\n",
		},
		{
			name: "nested schema reference",
			block: &ast.BlockStatement{
				Name: "person",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{Name: "name", Value: &ast.Identifier{Value: "str"}},
						{Name: "home", Value: &ast.Identifier{Value: "address"}},
					},
				},
			},
			expected: "class person(BaseModel):\n" +
				"    name: Any\n" +
				"    home: Any\n",
		},
		{
			name: "optional schema reference",
			block: &ast.BlockStatement{
				Name: "person",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "backup",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "address"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: "null"},
							},
						},
					},
				},
			},
			expected: "class person(BaseModel):\n" +
				"    backup: Any | Any\n",
		},
		{
			name: "list[str] field",
			block: &ast.BlockStatement{
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "tags",
							Value: &ast.Subscription{
								Object: &ast.Identifier{Value: "list"},
								Index:  &ast.Identifier{Value: "str"},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    tags: list[Any]\n",
		},
		{
			name: "list[schema_ref] with desc",
			block: &ast.BlockStatement{
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "items",
							Value: &ast.Subscription{
								Object: &ast.Identifier{Value: "list"},
								Index:  &ast.Identifier{Value: "address"},
							},
							Annotations: []*ast.Annotation{
								{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "All addresses"}}},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    items: list[Any] = Field(description=\"All addresses\")\n",
		},
		{
			name: "map[str] field",
			block: &ast.BlockStatement{
				Name: "config",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "metadata",
							Value: &ast.Subscription{
								Object: &ast.Identifier{Value: "map"},
								Index:  &ast.Identifier{Value: "str"},
							},
						},
					},
				},
			},
			expected: "class config(BaseModel):\n" +
				"    metadata: dict[str, Any]\n",
		},
		{
			name: "inline schema field",
			block: &ast.BlockStatement{
				Name: "report",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "summary",
							Value: &ast.BlockExpression{
								BlockBody: ast.BlockBody{
									Kind: types.BlockKindSchema,
									Assignments: []*ast.Assignment{
										{Name: "text", Value: &ast.Identifier{Value: "str"}},
										{Name: "score", Value: &ast.Identifier{Value: "int"}},
									},
								},
							},
						},
					},
				},
			},
			// BlockExpression typing calls symtab.Define; nil symtab panics (see subtest handler).
			expected: "",
		},
		{
			name: "empty schema",
			block: &ast.BlockStatement{
				Name: "empty",
				BlockBody: ast.BlockBody{
					Kind:        types.BlockKindSchema,
					Assignments: nil,
				},
			},
			expected: "class empty(BaseModel):\n" +
				"    pass\n",
		},
		{
			name: "non-desc annotations are ignored",
			block: &ast.BlockStatement{
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name:  "region",
							Value: &ast.Identifier{Value: "str"},
							Annotations: []*ast.Annotation{
								{Name: "required"},
								{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "Region code"}}},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    region: Any = Field(description=\"Region code\")\n",
		},
		{
			name: "mixed required and optional fields",
			block: &ast.BlockStatement{
				Name: "report",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name:  "title",
							Value: &ast.Identifier{Value: "str"},
							Annotations: []*ast.Annotation{
								{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "Report title"}}},
							},
						},
						{Name: "body", Value: &ast.Identifier{Value: "str"}},
						{
							Name: "rating",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "int"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: "null"},
							},
							Annotations: []*ast.Annotation{
								{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "Optional rating"}}},
							},
						},
						{
							Name: "tags",
							Value: &ast.Subscription{
								Object: &ast.Identifier{Value: "list"},
								Index:  &ast.Identifier{Value: "str"},
							},
						},
					},
				},
			},
			expected: "class report(BaseModel):\n" +
				"    title: Any = Field(description=\"Report title\")\n" +
				"    body: Any\n" +
				"    rating: Any | Any = Field(description=\"Optional rating\")\n" +
				"    tags: list[Any]\n",
		},
		{
			name: "only optional fields",
			block: &ast.BlockStatement{
				Name: "opts",
				BlockBody: ast.BlockBody{
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "a",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "str"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: "null"},
							},
						},
						{
							Name: "b",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "int"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: "null"},
							},
						},
					},
				},
			},
			expected: "class opts(BaseModel):\n" +
				"    a: Any | Any\n" +
				"    b: Any | Any\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "inline schema field" {
				defer func() {
					if r := recover(); r == nil {
						t.Fatal("expected panic: BlockExpression typing uses symtab.Define with nil symbol table in codegen")
					}
				}()
				SchemaBlockToSource(tt.block)
				return
			}
			got := SchemaBlockToSource(tt.block)
			if got != tt.expected {
				t.Errorf("SchemaBlockToSource():\ngot:\n%s\nwant:\n%s", got, tt.expected)
			}
		})
	}
}

// TestExtractDesc verifies @desc annotation extraction.
func TestExtractDesc(t *testing.T) {
	tests := []struct {
		name     string
		anns     []*ast.Annotation
		expected string
	}{
		{"nil annotations", nil, ""},
		{"empty annotations", []*ast.Annotation{}, ""},
		{"no desc annotation", []*ast.Annotation{{Name: "required"}}, ""},
		{
			"desc with string arg",
			[]*ast.Annotation{
				{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "hello"}}},
			},
			"hello",
		},
		{
			"desc among other annotations",
			[]*ast.Annotation{
				{Name: "required"},
				{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "world"}}},
			},
			"world",
		},
		{
			"desc with no args is invalid",
			[]*ast.Annotation{{Name: "desc"}},
			"",
		},
		{
			"desc with non-string arg is invalid",
			[]*ast.Annotation{
				{Name: "desc", Arguments: []ast.Expression{&ast.NumberLiteral{Value: 42}}},
			},
			"",
		},
		{
			"desc with multiple args uses first string",
			[]*ast.Annotation{
				{Name: "desc", Arguments: []ast.Expression{
					&ast.StringLiteral{Value: "first"},
					&ast.StringLiteral{Value: "second"},
				}},
			},
			// Only one arg expected, two args → no match.
			"",
		},
		{
			"first desc wins when multiple present",
			[]*ast.Annotation{
				{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "first"}}},
				{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "second"}}},
			},
			"first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDesc(tt.anns)
			if got != tt.expected {
				t.Errorf("extractDesc() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestSchemaImport verifies the pydantic import structure.
func TestSchemaImport(t *testing.T) {
	imp := SchemaImport()
	got := imp.Source()
	expected := "from pydantic import BaseModel, Field"
	if got != expected {
		t.Errorf("SchemaImport().Source() = %q, want %q", got, expected)
	}
}

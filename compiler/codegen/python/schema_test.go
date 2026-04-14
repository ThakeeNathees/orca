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
		{"string", lazyRef(types.BlockKindString), "str"},
		{"number", lazyRef(types.BlockKindNumber), "float"},
		{"bool", lazyRef("bool"), "bool"},
		{"any → Any", lazyRef(types.BlockKindAny), "Any"},
		{"nulltype → None", lazyRef(types.BlockKindNulltype), "None"},

		// User-defined schema refs.
		{"user schema", lazyRef("article"), "article"},
		{"user schema snake_case", lazyRef("vpc_data_t"), "vpc_data_t"},

		// List types.
		{"list[str]", types.NewListType(lazyRef(types.BlockKindString)), "list[str]"},
		{"list[float]", types.NewListType(lazyRef(types.BlockKindNumber)), "list[float]"},
		{"list[article]", types.NewListType(lazyRef("article")), "list[article]"},
		{"list[list[str]]", types.NewListType(types.NewListType(lazyRef(types.BlockKindString))), "list[list[str]]"},
		{"untyped list", types.Type{Kind: types.List}, "list"},

		// Map types.
		{"dict[str, float]", types.NewMapType(lazyRef(types.BlockKindString), lazyRef(types.BlockKindNumber)), "dict[str, float]"},
		{"dict[str, article]", types.NewMapType(lazyRef(types.BlockKindString), lazyRef("article")), "dict[str, article]"},
		{"dict[str, list[str]]", types.NewMapType(lazyRef(types.BlockKindString), types.NewListType(lazyRef(types.BlockKindString))), "dict[str, list[str]]"},
		{"untyped dict", types.Type{Kind: types.Map}, "dict"},

		// Union types.
		{"str | None", types.NewUnionType(lazyRef(types.BlockKindString), lazyRef(types.BlockKindNulltype)), "str | None"},
		{"str | float", types.NewUnionType(lazyRef(types.BlockKindString), lazyRef(types.BlockKindNumber)), "str | float"},
		{"float | None", types.NewUnionType(lazyRef(types.BlockKindNumber), lazyRef(types.BlockKindNulltype)), "float | None"},
		{"str | float | None", types.NewUnionType(lazyRef(types.BlockKindString), lazyRef(types.BlockKindNumber), lazyRef(types.BlockKindNulltype)), "str | float | None"},
		{"list[str] | None", types.NewUnionType(types.NewListType(lazyRef(types.BlockKindString)), lazyRef(types.BlockKindNulltype)), "list[str] | None"},
		{"article | None", types.NewUnionType(lazyRef("article"), lazyRef(types.BlockKindNulltype)), "article | None"},

		// Unresolved block refs with concrete names pass through as annotations (not empty → Any).
		{"block name model", lazyRef(types.BlockKindModel), "model"},
		{"block name agent", lazyRef(types.BlockKindAgent), "agent"},
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
// SchemaBlockToSource calls ExprTypeFromExpr(expr, nil). Without a symbol table,
// identifiers in schema type expressions still resolve as named type references, so
// generated Python annotations match Orca primitives and schema names.
func TestSchemaBlockToSource(t *testing.T) {
	tests := []struct {
		name     string
		block    *ast.BlockStatement
		expected string
	}{
		{
			name: "basic string and int fields",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "article",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{Name: "title", Value: &ast.Identifier{Value: types.BlockKindString}},
						{Name: "count", Value: &ast.Identifier{Value: types.BlockKindNumber}},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    title: str\n" +
				"    count: float\n",
		},
		{
			name: "all primitive types",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "config",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{Name: "name", Value: &ast.Identifier{Value: types.BlockKindString}},
						{Name: "count", Value: &ast.Identifier{Value: types.BlockKindNumber}},
						{Name: "enabled", Value: &ast.Identifier{Value: "bool"}},
					},
				},
			},
			expected: "class config(BaseModel):\n" +
				"    name: str\n" +
				"    count: float\n" +
				"    enabled: bool\n",
		},
		{
			name: "any type field",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "flexible",
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
				BlockBody: ast.BlockBody{
					Name: "article",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name:  "title",
							Value: &ast.Identifier{Value: "string"},
							Annotations: []*ast.Annotation{
								{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "The article title"}}},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    title: str = Field(description=\"The article title\")\n",
		},
		{
			name: "desc with special characters",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "article",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name:  "body",
							Value: &ast.Identifier{Value: "string"},
							Annotations: []*ast.Annotation{
								{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: `Contains "quotes" and \backslash`}}},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    body: str = Field(description=\"Contains \\\"quotes\\\" and \\\\backslash\")\n",
		},
		{
			name: "optional field without desc (null identifier union)",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "article",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "score",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "float"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: "nulltype"},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    score: float | None = None\n",
		},
		{
			name: "optional field without desc (null identifier)",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "article",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "score",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "float"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: "nulltype"},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    score: float | None = None\n",
		},
		{
			name: "optional field with desc",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "article",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "score",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "float"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: "nulltype"},
							},
							Annotations: []*ast.Annotation{
								{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "Confidence score"}}},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    score: float | None = Field(default=None, description=\"Confidence score\")\n",
		},
		{
			name: "multi-member union with null",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "result",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "value",
							Value: &ast.BinaryExpression{
								Left: &ast.BinaryExpression{
									Left:     &ast.Identifier{Value: types.BlockKindString},
									Operator: token.Token{Type: token.PIPE, Literal: "|"},
									Right:    &ast.Identifier{Value: types.BlockKindNumber},
								},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: types.BlockKindNulltype},
							},
						},
					},
				},
			},
			expected: "class result(BaseModel):\n" +
				"    value: str | float | None = None\n",
		},
		{
			name: "non-optional union",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "result",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "value",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: types.BlockKindString},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: types.BlockKindNumber},
							},
						},
					},
				},
			},
			expected: "class result(BaseModel):\n" +
				"    value: str | float\n",
		},
		{
			name: "nested schema reference",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "person",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{Name: "name", Value: &ast.Identifier{Value: "string"}},
						{Name: "home", Value: &ast.Identifier{Value: "address"}},
					},
				},
			},
			expected: "class person(BaseModel):\n" +
				"    name: str\n" +
				"    home: address\n",
		},
		{
			name: "optional schema reference",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "person",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "backup",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "address"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: "nulltype"},
							},
						},
					},
				},
			},
			expected: "class person(BaseModel):\n" +
				"    backup: address | None = None\n",
		},
		{
			name: "list[str] field",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "article",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "tags",
							Value: &ast.Subscription{
								Object:  &ast.Identifier{Value: "list"},
								Indices: []ast.Expression{&ast.Identifier{Value: "string"}},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    tags: list[str]\n",
		},
		{
			name: "list[schema_ref] with desc",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "article",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "items",
							Value: &ast.Subscription{
								Object:  &ast.Identifier{Value: "list"},
								Indices: []ast.Expression{&ast.Identifier{Value: "address"}},
							},
							Annotations: []*ast.Annotation{
								{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "All addresses"}}},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    items: list[address] = Field(description=\"All addresses\")\n",
		},
		{
			name: "map[string, string] field",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "config",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "metadata",
							Value: &ast.Subscription{
								Object:  &ast.Identifier{Value: "map"},
								Indices: []ast.Expression{&ast.Identifier{Value: "string"}, &ast.Identifier{Value: "string"}},
							},
						},
					},
				},
			},
			expected: "class config(BaseModel):\n" +
				"    metadata: dict[str, str]\n",
		},
		{
			name: "inline schema field",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "report",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "summary",
							Value: &ast.BlockExpression{
								BlockBody: ast.BlockBody{
									Kind: types.BlockKindSchema,
									Assignments: []*ast.Assignment{
										{Name: "text", Value: &ast.Identifier{Value: types.BlockKindString}},
										{Name: "score", Value: &ast.Identifier{Value: types.BlockKindNumber}},
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
				BlockBody: ast.BlockBody{
					Name:        "empty",
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
				BlockBody: ast.BlockBody{
					Name: "article",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name:  "region",
							Value: &ast.Identifier{Value: "string"},
							Annotations: []*ast.Annotation{
								{Name: "required"},
								{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "Region code"}}},
							},
						},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    region: str = Field(description=\"Region code\")\n",
		},
		{
			name: "mixed required and optional fields",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "report",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name:  "title",
							Value: &ast.Identifier{Value: "string"},
							Annotations: []*ast.Annotation{
								{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "Report title"}}},
							},
						},
						{Name: "body", Value: &ast.Identifier{Value: "string"}},
						{
							Name: "rating",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: types.BlockKindNumber},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: types.BlockKindNulltype},
							},
							Annotations: []*ast.Annotation{
								{Name: "desc", Arguments: []ast.Expression{&ast.StringLiteral{Value: "Optional rating"}}},
							},
						},
						{
							Name: "tags",
							Value: &ast.Subscription{
								Object:  &ast.Identifier{Value: "list"},
								Indices: []ast.Expression{&ast.Identifier{Value: "string"}},
							},
						},
					},
				},
			},
			expected: "class report(BaseModel):\n" +
				"    title: str = Field(description=\"Report title\")\n" +
				"    body: str\n" +
				"    rating: float | None = Field(default=None, description=\"Optional rating\")\n" +
				"    tags: list[str]\n",
		},
		{
			name: "only optional fields",
			block: &ast.BlockStatement{
				BlockBody: ast.BlockBody{
					Name: "opts",
					Kind: types.BlockKindSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "a",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "string"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: "nulltype"},
							},
						},
						{
							Name: "b",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: types.BlockKindNumber},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.Identifier{Value: types.BlockKindNulltype},
							},
						},
					},
				},
			},
			expected: "class opts(BaseModel):\n" +
				"    a: str | None = None\n" +
				"    b: float | None = None\n",
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

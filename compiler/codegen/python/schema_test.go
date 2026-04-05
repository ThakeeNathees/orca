package python

import (
	"strings"
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
	"github.com/thakee/orca/compiler/types"
)

// TestOrcaTypeToPythonTypeName verifies resolved Type to Python annotation conversion.
func TestOrcaTypeToPythonTypeName(t *testing.T) {
	tests := []struct {
		name     string
		typ      types.Type
		expected string
	}{
		// Primitives.
		{"str", types.Str(), "str"},
		{"int", types.Int(), "int"},
		{"float", types.Float(), "float"},
		{"bool", types.Bool(), "bool"},
		{"any → Any", types.Any(), "Any"},
		{"null → None", types.Null(), "None"},

		// User-defined schema refs.
		{"user schema", types.CreateSchema("article"), "article"},
		{"user schema snake_case", types.CreateSchema("vpc_data_t"), "vpc_data_t"},

		// List types.
		{"list[str]", types.NewListType(types.Str()), "list[str]"},
		{"list[int]", types.NewListType(types.Int()), "list[int]"},
		{"list[article]", types.NewListType(types.CreateSchema("article")), "list[article]"},
		{"list[list[str]]", types.NewListType(types.NewListType(types.Str())), "list[list[str]]"},
		{"untyped list", types.Type{Kind: types.List}, "list"},

		// Map types.
		{"dict[str, int]", types.NewMapType(types.Str(), types.Int()), "dict[str, int]"},
		{"dict[str, article]", types.NewMapType(types.Str(), types.CreateSchema("article")), "dict[str, article]"},
		{"dict[str, list[str]]", types.NewMapType(types.Str(), types.NewListType(types.Str())), "dict[str, list[str]]"},
		{"untyped dict", types.Type{Kind: types.Map}, "dict"},

		// Union types.
		{"str | None", types.NewUnionType(types.Str(), types.Null()), "str | None"},
		{"str | int", types.NewUnionType(types.Str(), types.Int()), "str | int"},
		{"float | None", types.NewUnionType(types.Float(), types.Null()), "float | None"},
		{"str | int | None", types.NewUnionType(types.Str(), types.Int(), types.Null()), "str | int | None"},
		{"str | int | float", types.NewUnionType(types.Str(), types.Int(), types.Float()), "str | int | float"},
		{"list[str] | None", types.NewUnionType(types.NewListType(types.Str()), types.Null()), "list[str] | None"},
		{"article | None", types.NewUnionType(types.CreateSchema("article"), types.Null()), "article | None"},

		// Generic block ref (non-schema) — e.g. bare "model" type.
		{"generic block ref model", types.NewBlockRefType(token.BlockModel), "Any"},
		{"generic block ref agent", types.NewBlockRefType(token.BlockAgent), "Any"},
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
					Kind: token.BlockSchema,
					Assignments: []*ast.Assignment{
						{Name: "title", Value: &ast.Identifier{Value: "str"}},
						{Name: "count", Value: &ast.Identifier{Value: "int"}},
					},
				},
			},
			expected: "class article(BaseModel):\n" +
				"    title: str\n" +
				"    count: int\n",
		},
		{
			name: "all primitive types",
			block: &ast.BlockStatement{
				Name: "config",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
					Assignments: []*ast.Assignment{
						{Name: "name", Value: &ast.Identifier{Value: "str"}},
						{Name: "count", Value: &ast.Identifier{Value: "int"}},
						{Name: "rate", Value: &ast.Identifier{Value: "float"}},
						{Name: "enabled", Value: &ast.Identifier{Value: "bool"}},
					},
				},
			},
			expected: "class config(BaseModel):\n" +
				"    name: str\n" +
				"    count: int\n" +
				"    rate: float\n" +
				"    enabled: bool\n",
		},
		{
			name: "any type field",
			block: &ast.BlockStatement{
				Name: "flexible",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
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
					Kind: token.BlockSchema,
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
				"    title: str = Field(description=\"The article title\")\n",
		},
		{
			name: "desc with special characters",
			block: &ast.BlockStatement{
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
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
				"    body: str = Field(description=\"Contains \\\"quotes\\\" and \\\\backslash\")\n",
		},
		{
			name: "optional field without desc (NullLiteral)",
			block: &ast.BlockStatement{
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "score",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "float"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.NullLiteral{},
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
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
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
				"    score: float | None = None\n",
		},
		{
			name: "optional field with desc",
			block: &ast.BlockStatement{
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "score",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "float"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.NullLiteral{},
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
				Name: "result",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
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
								Right:    &ast.NullLiteral{},
							},
						},
					},
				},
			},
			expected: "class result(BaseModel):\n" +
				"    value: str | int | None = None\n",
		},
		{
			name: "non-optional union",
			block: &ast.BlockStatement{
				Name: "result",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
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
				"    value: str | int\n",
		},
		{
			name: "nested schema reference",
			block: &ast.BlockStatement{
				Name: "person",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
					Assignments: []*ast.Assignment{
						{Name: "name", Value: &ast.Identifier{Value: "str"}},
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
				Name: "person",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "backup",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "address"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.NullLiteral{},
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
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
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
				"    tags: list[str]\n",
		},
		{
			name: "list[schema_ref] with desc",
			block: &ast.BlockStatement{
				Name: "article",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
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
				"    items: list[address] = Field(description=\"All addresses\")\n",
		},
		{
			name: "map[str] field",
			block: &ast.BlockStatement{
				Name: "config",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
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
				"    metadata: dict[str, str]\n",
		},
		{
			name: "inline schema field",
			block: &ast.BlockStatement{
				Name: "report",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "summary",
							Value: &ast.BlockExpression{
								BlockBody: ast.BlockBody{
									Kind: token.BlockSchema,
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
			// Inline schemas get synthetic __anon_N names from the type system.
			// We use empty expected and check via a custom assertion below.
			expected: "",
		},
		{
			name: "empty schema",
			block: &ast.BlockStatement{
				Name: "empty",
				BlockBody: ast.BlockBody{
					Kind:        token.BlockSchema,
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
					Kind: token.BlockSchema,
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
				"    region: str = Field(description=\"Region code\")\n",
		},
		{
			name: "mixed required and optional fields",
			block: &ast.BlockStatement{
				Name: "report",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
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
								Right:    &ast.NullLiteral{},
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
				"    title: str = Field(description=\"Report title\")\n" +
				"    body: str\n" +
				"    rating: int | None = Field(default=None, description=\"Optional rating\")\n" +
				"    tags: list[str]\n",
		},
		{
			name: "only optional fields",
			block: &ast.BlockStatement{
				Name: "opts",
				BlockBody: ast.BlockBody{
					Kind: token.BlockSchema,
					Assignments: []*ast.Assignment{
						{
							Name: "a",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "str"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.NullLiteral{},
							},
						},
						{
							Name: "b",
							Value: &ast.BinaryExpression{
								Left:     &ast.Identifier{Value: "int"},
								Operator: token.Token{Type: token.PIPE, Literal: "|"},
								Right:    &ast.NullLiteral{},
							},
						},
					},
				},
			},
			expected: "class opts(BaseModel):\n" +
				"    a: str | None = None\n" +
				"    b: int | None = None\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SchemaBlockToSource(tt.block)
			if tt.name == "inline schema field" {
				// Inline schemas get __anon_N names with a global counter,
				// so we check the structure rather than exact name.
				if !strings.Contains(got, "class report(BaseModel):") {
					t.Errorf("missing class header in:\n%s", got)
				}
				if !strings.Contains(got, "summary: __anon_") {
					t.Errorf("expected summary field with __anon_ type, got:\n%s", got)
				}
				return
			}
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
				{Name: "desc", Arguments: []ast.Expression{&ast.IntegerLiteral{Value: 42}}},
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

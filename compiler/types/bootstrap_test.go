package types

import (
	_ "embed"
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
	"github.com/thakee/orca/compiler/token"
)

//go:embed bootstrap.oc
var testBootstrapSource string

// typePtr returns a pointer to a Type for use in test table comparisons.
func typePtr(t Type) *Type { return &t }

// TestLoadSchemas verifies that the embedded builtins.oc is parsed and
// all expected schemas are present with the correct number of fields.
func TestLoadSchemas(t *testing.T) {
	res := Bootstrap(testBootstrapSource)
	schemas := make(map[string]BlockSchema)
	for _, s := range res.Schemas {
		schemas[s.BlockName] = s
	}

	tests := []struct {
		name      string
		blockType string
		numFields int
	}{
		{"str", "str", 0},
		{"number", "number", 0},
		{"bool", "bool", 0},
		{"list", "list", 0},
		{"map", "map", 0},
		{"any", "any", 0},
		{"null", "null", 0},
		{"model", "model", 5},
		{"agent", "agent", 4},
		{"tool", "tool", 4},
		{"knowledge", "knowledge", 1},
		{"workflow", "workflow", 2},
		{"input", "input", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, ok := schemas[tt.blockType]
			if !ok {
				t.Fatalf("schema %q not found", tt.blockType)
			}
			if len(schema.Fields) != tt.numFields {
				t.Errorf("num fields = %d, want %d", len(schema.Fields), tt.numFields)
			}
		})
	}
}

// TestLoadSchemasFieldTypes verifies that field types are correctly
// resolved from the .oc file, including union types and block references.
func TestLoadSchemasFieldTypes(t *testing.T) {
	res := Bootstrap(testBootstrapSource)
	schemas := make(map[string]BlockSchema)
	for _, s := range res.Schemas {
		schemas[s.BlockName] = s
	}

	// wantString is the canonical Orca type string Type.String() should produce (lazy refs must print their name, not <type:blockref>).
	tests := []struct {
		name     string
		block    string
		field    string
		kind     TypeKind
		required bool
		wantStr  string
	}{
		{"model.provider", "model", "provider", BlockRef, true, "str"},
		{"model.model_name", "model", "model_name", Union, true, "str | model"},
		{"model.temperature", "model", "temperature", BlockRef, false, "float"},
		{"agent.model", "agent", "model", Union, true, "str | model"},
		{"agent.persona", "agent", "persona", BlockRef, true, "str"},
		{"agent.tools", "agent", "tools", List, false, "list[tool]"},
		{"tool.desc", "tool", "desc", BlockRef, false, "str"},
		{"tool.invoke", "tool", "invoke", BlockRef, true, "str"},
		{"workflow.name", "workflow", "name", BlockRef, false, "str"},
		{"input.type", "input", "type", BlockRef, true, "schema"},
		{"input.desc", "input", "desc", BlockRef, false, "str"},
		{"input.default", "input", "default", BlockRef, false, "any"},
		{"input.sensitive", "input", "sensitive", BlockRef, false, "bool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := schemas[tt.block].Fields[tt.field]
			if !ok {
				t.Fatalf("field %s.%s not found", tt.block, tt.field)
			}
			if field.Type.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", field.Type.Kind, tt.kind)
			}
			if got := field.Type.String(); got != tt.wantStr {
				t.Errorf("Type.String() = %q, want %q", got, tt.wantStr)
			}
			if field.Required != tt.required {
				t.Errorf("Required = %v, want %v", field.Required, tt.required)
			}
		})
	}
}

// TestLoadSchemasDescriptions verifies that @desc annotations are
// correctly extracted into FieldSchema.Description.
func TestLoadSchemasDescriptions(t *testing.T) {
	res := Bootstrap(testBootstrapSource)
	schemas := make(map[string]BlockSchema)
	for _, s := range res.Schemas {
		schemas[s.BlockName] = s
	}

	tests := []struct {
		name     string
		block    string
		field    string
		wantDesc bool
	}{
		{"model.provider has desc", "model", "provider", true},
		{"agent.model has desc", "agent", "model", true},
		{"tool.invoke has desc", "tool", "invoke", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := schemas[tt.block].Fields[tt.field]
			if !ok {
				t.Fatalf("field %s.%s not found", tt.block, tt.field)
			}
			hasDesc := field.Description != ""
			if hasDesc != tt.wantDesc {
				t.Errorf("has description = %v, want %v (desc=%q)", hasDesc, tt.wantDesc, field.Description)
			}
		})
	}
}

// TestResolveIdentAsType verifies that ExprType on an identifier (bootstrap mode)
// maps every name to a BlockRef via NewBlockRefType — the same path as identType.
func TestResolveIdentAsType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Type
	}{
		{"primitive str", "str", NewBlockRefType("str", nil)},
		{"primitive number", "number", NewBlockRefType("number", nil)},
		{"primitive bool", "bool", NewBlockRefType("bool", nil)},
		{"primitive any", "any", NewBlockRefType("any", nil)},
		{"block type model", "model", NewBlockRefType("model", nil)},
		{"block type agent", "agent", NewBlockRefType("agent", nil)},
		{"block type cron", "cron", NewBlockRefType("cron", nil)},
		{"block type webhook", "webhook", NewBlockRefType("webhook", nil)},
		{"block type schema", "schema", NewBlockRefType("schema", nil)},
		{"user schema name", "vpc_data_t", NewBlockRefType("vpc_data_t", nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := ExprType(&ast.Identifier{Value: tt.input}, nil)
			if typ.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", typ.Kind)
			}
			if !typ.Equals(tt.expected) {
				t.Errorf("ExprType(ident %q) = %s, want %s", tt.input, typ.String(), tt.expected.String())
			}
		})
	}
}

// TestExprTypeBootstrap verifies that ExprType with nil symbols (bootstrap
// mode) handles all type expression forms correctly.
func TestExprTypeBootstrap(t *testing.T) {
	anyTyp := IdentType("any", nil)
	tests := []struct {
		name    string
		expr    ast.Expression
		kind    TypeKind
		expType *Type // expected type to check with Equals (nil to skip)
	}{
		{
			"null literal resolves to null type",
			&ast.NullLiteral{},
			BlockRef, typePtr(IdentType("null", nil)),
		},
		{
			"identifier str resolves to str",
			&ast.Identifier{Value: "str"},
			BlockRef, typePtr(NewBlockRefType("str", nil)),
		},
		{
			"identifier model resolves to model ref",
			&ast.Identifier{Value: "model"},
			BlockRef, typePtr(NewBlockRefType("model", nil)),
		},
		{
			"subscription list[str] resolves to list type",
			&ast.Subscription{
				Object: &ast.Identifier{Value: "list"},
				Index:  &ast.Identifier{Value: "str"},
			},
			List, nil,
		},
		{
			"subscription map[int] resolves to map type",
			&ast.Subscription{
				Object: &ast.Identifier{Value: "map"},
				Index:  &ast.Identifier{Value: "int"},
			},
			Map, nil,
		},
		{
			"unsupported parameterized type returns any",
			&ast.Subscription{
				Object: &ast.Identifier{Value: "set"},
				Index:  &ast.Identifier{Value: "str"},
			},
			BlockRef, typePtr(anyTyp),
		},
		{
			"union via binary pipe expression",
			&ast.BinaryExpression{
				Left:     &ast.Identifier{Value: "str"},
				Operator: token.Token{Type: token.PIPE},
				Right:    &ast.Identifier{Value: "int"},
			},
			Union, nil,
		},
		{
			"binary expression with non-pipe operator returns any",
			&ast.BinaryExpression{
				Left:     &ast.Identifier{Value: "str"},
				Operator: token.Token{Type: token.PLUS, Literal: "+"},
				Right:    &ast.Identifier{Value: "int"},
			},
			BlockRef, typePtr(anyTyp),
		},
		{
			"number literal returns number",
			&ast.NumberLiteral{Value: 42},
			BlockRef, typePtr(IdentType("number", nil)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := ExprType(tt.expr, nil)
			if typ.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", typ.Kind, tt.kind)
			}
			if tt.expType != nil && !typ.Equals(*tt.expType) {
				t.Errorf("Type = %s, want %s", typ.String(), tt.expType.String())
			}
		})
	}
}

// TestExprTypeInlineSchema verifies that inline schema expressions
// are resolved and registered with a synthetic name.
func TestExprTypeInlineSchema(t *testing.T) {
	expr := &ast.BlockExpression{
		BlockBody: ast.BlockBody{
			Kind: BlockKindSchema,
			Assignments: []*ast.Assignment{
				{
					Name:  "host",
					Value: &ast.Identifier{Value: "str"},
				},
			},
		},
	}

	typ := ExprType(expr, nil)
	if typ.Kind != BlockRef {
		t.Errorf("Kind = %v, want BlockRef", typ.Kind)
	}
	if typ.Block == nil {
		t.Fatal("expected Block to be set for inline schema")
	}
	if typ.Block.Ast == nil || typ.Block.Ast.Kind != BlockKindSchema {
		t.Errorf("Ast.Kind = %v, want %q", typ.Block.Ast.Kind, BlockKindSchema)
	}
	if typ.BlockName == "" {
		t.Error("expected non-empty BlockName for inline schema")
	}

	if _, hasHost := typ.Block.Fields["host"]; !hasHost {
		t.Error("expected inline schema to have field 'host'")
	}
}

// TestNullStrippingInUnion verifies NewFieldSchema strips | null using Type.IsNull()
// (including lazy identifier "null") and marks the field optional.
func TestNullStrippingInUnion(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		required    bool
		resultKind  TypeKind
		expType     *Type // expected type to check with Equals (nil to skip)
		memberCount int
	}{
		{
			"str | null strips to str, optional",
			"schema test_strip {\n  @suppress(\"duplicate-block\")\n  field = str | null\n}",
			false, BlockRef, typePtr(NewBlockRefType("str", nil)), 0,
		},
		{
			"str | model | null strips null, two-member union",
			"schema test_strip2 {\n  @suppress(\"duplicate-block\")\n  field = str | model | null\n}",
			false, Union, nil, 2,
		},
		{
			"str (no union) is required BlockRef",
			"schema test_strip3 {\n  @suppress(\"duplicate-block\")\n  field = str\n}",
			true, BlockRef, typePtr(NewBlockRefType("str", nil)), 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input, "")
			p := parser.New(l)
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				t.Fatalf("parse errors: %v", p.Errors())
			}
			block := program.Statements[0].(*ast.BlockStatement)
			assign := block.Assignments[0]
			fs := NewFieldSchema(assign, nil)
			if fs.Required != tt.required {
				t.Errorf("Required = %v, want %v", fs.Required, tt.required)
			}
			if fs.Type.Kind != tt.resultKind {
				t.Errorf("Kind = %v, want %v", fs.Type.Kind, tt.resultKind)
			}
			if tt.expType != nil && !fs.Type.Equals(*tt.expType) {
				t.Errorf("Type = %s, want %s", fs.Type.String(), tt.expType.String())
			}
			if tt.memberCount > 0 && len(fs.Type.Members) != tt.memberCount {
				t.Errorf("Members = %d, want %d", len(fs.Type.Members), tt.memberCount)
			}
		})
	}
}

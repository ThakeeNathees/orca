package types

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
	"github.com/thakee/orca/compiler/token"
)

// typePtr returns a pointer to a Type for use in test table comparisons.
func typePtr(t Type) *Type { return &t }

// TestLoadSchemas verifies that the embedded builtins.oc is parsed and
// all expected schemas are present with the correct number of fields.
func TestLoadSchemas(t *testing.T) {
	schemas, err := Bootstrap()
	if err != nil {
		t.Fatalf("loadSchemas() error: %v", err)
	}

	tests := []struct {
		name      string
		blockType string
		numFields int
	}{
		{"str", "str", 0},
		{"int", "int", 0},
		{"float", "float", 0},
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
	schemas, err := Bootstrap()
	if err != nil {
		t.Fatalf("loadSchemas() error: %v", err)
	}

	tests := []struct {
		name     string
		block    string
		field    string
		kind     TypeKind // expected Kind
		expType  *Type    // expected type (for BlockRef, nil to skip)
		required bool
	}{
		// Simple identifiers in builtins resolve to NewBlockRefType (bootstrap ExprType).
		{"model.provider", "model", "provider", BlockRef, typePtr(NewBlockRefType(BlockKindSchema, "str")), true},
		{"model.model_name", "model", "model_name", Union, nil, true},
		// float | null — union until identifier "null" is treated as IsNull() in ResolveFieldSchema.
		{"model.temperature", "model", "temperature", Union, nil, true},
		{"agent.model", "agent", "model", Union, nil, true},
		{"agent.persona", "agent", "persona", BlockRef, typePtr(NewBlockRefType(BlockKindSchema, "str")), true},
		{"agent.tools", "agent", "tools", Union, nil, true},
		{"tool.desc", "tool", "desc", Union, nil, true},
		{"tool.invoke", "tool", "invoke", BlockRef, typePtr(NewBlockRefType(BlockKindSchema, "str")), true},
		{"workflow.name", "workflow", "name", Union, nil, true},
		{"input.type", "input", "type", BlockRef, typePtr(NewBlockRefType(BlockKindSchema, "input")), true},
		{"input.desc", "input", "desc", Union, nil, true},
		{"input.default", "input", "default", Union, nil, true},
		{"input.sensitive", "input", "sensitive", Union, nil, true},
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
			if tt.expType != nil && !field.Type.Equals(*tt.expType) {
				t.Errorf("Type = %s, want %s", field.Type.String(), tt.expType.String())
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
	schemas, err := Bootstrap()
	if err != nil {
		t.Fatalf("loadSchemas() error: %v", err)
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
		{"primitive str", "str", NewBlockRefType(BlockKindSchema, "str")},
		{"primitive int", "int", NewBlockRefType(BlockKindSchema, "int")},
		{"primitive float", "float", NewBlockRefType(BlockKindSchema, "float")},
		{"primitive bool", "bool", NewBlockRefType(BlockKindSchema, "bool")},
		{"primitive any", "any", NewBlockRefType(BlockKindSchema, "any")},
		{"block type model", "model", NewBlockRefType(BlockKindSchema, "model")},
		{"block type agent", "agent", NewBlockRefType(BlockKindSchema, "agent")},
		{"block type cron", "cron", NewBlockRefType(BlockKindSchema, "cron")},
		{"block type webhook", "webhook", NewBlockRefType(BlockKindSchema, "webhook")},
		{"block type schema", "schema", NewBlockRefType(BlockKindSchema, "schema")},
		{"user schema name", "vpc_data_t", NewBlockRefType(BlockKindSchema, "vpc_data_t")},
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
	tests := []struct {
		name    string
		expr    ast.Expression
		kind    TypeKind
		expType *Type // expected type to check with Equals (nil to skip)
	}{
		{
			"null literal resolves to null type",
			&ast.NullLiteral{},
			BlockRef, typePtr(Null()),
		},
		{
			"identifier str resolves to str",
			&ast.Identifier{Value: "str"},
			BlockRef, typePtr(NewBlockRefType(BlockKindSchema, "str")),
		},
		{
			"identifier model resolves to model ref",
			&ast.Identifier{Value: "model"},
			BlockRef, typePtr(NewBlockRefType(BlockKindSchema, "model")),
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
			BlockRef, typePtr(Any()),
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
			BlockRef, typePtr(Any()),
		},
		{
			"integer literal returns int",
			&ast.IntegerLiteral{Value: 42},
			BlockRef, typePtr(Int()),
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
	if typ.BlockKind != BlockKindSchema {
		t.Errorf("BlockKind = %v, want %q", typ.BlockKind, BlockKindSchema)
	}
	if typ.BlockName == "" {
		t.Error("expected non-empty SchemaName for inline schema")
	}

	// The registered schema should be accessible.
	schema, ok := GetSchema(typ.BlockName)
	if !ok {
		t.Fatal("expected inline schema to be registered")
	}
	if _, hasHost := schema.Fields["host"]; !hasHost {
		t.Error("expected inline schema to have field 'host'")
	}
}

// TestNullStrippingInUnion verifies ResolveFieldSchema behavior on unions.
// Identifier `null` in `x | null` is currently a plain identifier (not the null
// type), so optionality stripping does not apply until that is unified.
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
			"str | null stays union (null not stripped as keyword yet)",
			"schema test_strip {\n  @suppress(\"duplicate-block\")\n  field = str | null\n}",
			true, Union, nil, 2,
		},
		{
			"str | model | null is a three-member union",
			"schema test_strip2 {\n  @suppress(\"duplicate-block\")\n  field = str | model | null\n}",
			true, Union, nil, 3,
		},
		{
			"str (no union) is required BlockRef",
			"schema test_strip3 {\n  @suppress(\"duplicate-block\")\n  field = str\n}",
			true, BlockRef, typePtr(NewBlockRefType(BlockKindSchema, "str")), 0,
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
			fs := NewFieldSchema(assign)
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

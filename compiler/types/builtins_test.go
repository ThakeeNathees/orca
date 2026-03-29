package types

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
	"github.com/thakee/orca/compiler/token"
)

// typePtr returns a pointer to a Type value for use in test tables.
func typePtr(t Type) *Type { return &t }

// TestLoadSchemas verifies that the embedded builtins.oc file
// is parsed and produces the expected schema map.
func TestLoadSchemas(t *testing.T) {
	schemas, err := loadSchemas()
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
		{"knowledge", "knowledge", 2},
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
	schemas, err := loadSchemas()
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
		{"model.provider", "model", "provider", BlockRef, typePtr(Str()), true},
		{"model.model_name", "model", "model_name", Union, nil, true},
		{"model.temperature", "model", "temperature", BlockRef, typePtr(Float()), false},
		{"agent.model", "agent", "model", Union, nil, true},
		{"agent.persona", "agent", "persona", BlockRef, typePtr(Str()), true},
		{"agent.tools", "agent", "tools", List, nil, false},
		{"tool.name", "tool", "name", BlockRef, typePtr(Str()), true},
		{"tool.desc", "tool", "desc", BlockRef, typePtr(Str()), false},
		{"knowledge.name", "knowledge", "name", BlockRef, typePtr(Str()), true},
		{"workflow.name", "workflow", "name", BlockRef, typePtr(Str()), false},
		{"input.type", "input", "type", BlockRef, typePtr(TypeOf(token.BlockSchema)), true},
		{"input.desc", "input", "desc", BlockRef, typePtr(Str()), false},
		{"input.default", "input", "default", BlockRef, typePtr(Any()), false},
		{"input.sensitive", "input", "sensitive", BlockRef, typePtr(Bool()), false},
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

// TestTokenTypeToBlockKindSchema verifies that schema token type maps to BlockSchema.
func TestTokenTypeToBlockKindSchema(t *testing.T) {
	kind, ok := token.TokenTypeToBlockKind(token.SCHEMA)
	if !ok {
		t.Fatal("expected TokenTypeToBlockKind(SCHEMA) to return true")
	}
	if kind != token.BlockSchema {
		t.Errorf("kind = %v, want %v", kind, token.BlockSchema)
	}
}

// TestLoadSchemasUnionMembers verifies that union types contain the
// correct member types.
func TestLoadSchemasUnionMembers(t *testing.T) {
	schemas, err := loadSchemas()
	if err != nil {
		t.Fatalf("loadSchemas() error: %v", err)
	}

	// model.model_name should be str | model (required).
	field := schemas["model"].Fields["model_name"]
	if field.Type.Kind != Union {
		t.Fatalf("expected Union, got %v", field.Type.Kind)
	}
	if len(field.Type.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(field.Type.Members))
	}
	if !field.Type.Members[0].Equals(Str()) {
		t.Errorf("first member = %s, want str", field.Type.Members[0].String())
	}
	if field.Type.Members[1].BlockKind != token.BlockModel {
		t.Errorf("second member BlockKind = %v, want %v", field.Type.Members[1].BlockKind, token.BlockModel)
	}
}

// TestLoadSchemasParameterizedList verifies that list[tool] resolves to
// a List type with a BlockRef(tool) element type.
func TestLoadSchemasParameterizedList(t *testing.T) {
	schemas, err := loadSchemas()
	if err != nil {
		t.Fatalf("loadSchemas() error: %v", err)
	}

	field, ok := schemas["agent"].Fields["tools"]
	if !ok {
		t.Fatalf("field agent.tools not found")
	}
	if field.Type.Kind != List {
		t.Fatalf("expected List, got %v", field.Type.Kind)
	}
	if field.Type.ElementType == nil {
		t.Fatalf("expected ElementType to be set")
	}
	if field.Type.ElementType.Kind != BlockRef {
		t.Errorf("ElementType.Kind = %v, want BlockRef", field.Type.ElementType.Kind)
	}
	if field.Type.ElementType.BlockKind != token.BlockTool {
		t.Errorf("ElementType.BlockKind = %v, want %v", field.Type.ElementType.BlockKind, token.BlockTool)
	}
}

// TestLoadSchemasFieldDescription verifies that the optional desc property
// is correctly loaded from schema definitions.
func TestLoadSchemasFieldDescription(t *testing.T) {
	schemas, err := loadSchemas()
	if err != nil {
		t.Fatalf("loadSchemas() error: %v", err)
	}

	tests := []struct {
		name      string
		blockType string
		fieldName string
		hasDesc   bool
	}{
		{"model.provider has desc", "model", "provider", true},
		{"agent.model has desc", "agent", "model", true},
		{"agent.tools has desc", "agent", "tools", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := schemas[tt.blockType].Fields[tt.fieldName]
			if !ok {
				t.Fatalf("field %s.%s not found", tt.blockType, tt.fieldName)
			}
			if tt.hasDesc && field.Description == "" {
				t.Error("expected non-empty Description")
			}
			if !tt.hasDesc && field.Description != "" {
				t.Errorf("Description = %q, want empty", field.Description)
			}
		})
	}
}

// TestResolveIdentTypeUnified verifies that resolveIdentType treats
// all names uniformly — primitives, block types, and unknown names
// all produce BlockRef types.
func TestResolveIdentTypeUnified(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Type
		wantErr  bool
	}{
		{"primitive str", "str", Str(), false},
		{"primitive int", "int", Int(), false},
		{"primitive float", "float", Float(), false},
		{"primitive bool", "bool", Bool(), false},
		{"primitive any", "any", Any(), false},
		{"block type model", "model", TypeOf(token.BlockModel), false},
		{"block type agent", "agent", TypeOf(token.BlockAgent), false},
		{"block type schema", "schema", TypeOf(token.BlockSchema), false},
		{"user schema name", "vpc_data_t", SchemaTypeOf("vpc_data_t"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ, err := resolveIdentType(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if typ.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", typ.Kind)
			}
			if !typ.Equals(tt.expected) {
				t.Errorf("resolveIdentType(%q) = %s, want %s", tt.input, typ.String(), tt.expected.String())
			}
		})
	}
}

// TestResolveTypeExpr verifies that ResolveTypeExpr handles all expression
// types: identifiers, subscriptions, null literals, unions, inline schemas,
// and produces errors for unsupported expressions.
func TestResolveTypeExpr(t *testing.T) {
	tests := []struct {
		name    string
		expr    ast.Expression
		kind    TypeKind
		expType *Type // expected type to check with Equals (nil to skip)
		wantErr bool
	}{
		{
			"null literal resolves to null type",
			&ast.NullLiteral{},
			BlockRef, typePtr(Null()), false,
		},
		{
			"identifier str resolves to str",
			&ast.Identifier{Value: "str"},
			BlockRef, typePtr(Str()), false,
		},
		{
			"identifier model resolves to model ref",
			&ast.Identifier{Value: "model"},
			BlockRef, typePtr(TypeOf(token.BlockModel)), false,
		},
		{
			"subscription list[str] resolves to list type",
			&ast.Subscription{
				Object: &ast.Identifier{Value: "list"},
				Index:  &ast.Identifier{Value: "str"},
			},
			List, nil, false,
		},
		{
			"subscription map[int] resolves to map type",
			&ast.Subscription{
				Object: &ast.Identifier{Value: "map"},
				Index:  &ast.Identifier{Value: "int"},
			},
			Map, nil, false,
		},
		{
			"unsupported parameterized type errors",
			&ast.Subscription{
				Object: &ast.Identifier{Value: "set"},
				Index:  &ast.Identifier{Value: "str"},
			},
			0, nil, true,
		},
		{
			"subscription with non-identifier base errors",
			&ast.Subscription{
				Object: &ast.IntegerLiteral{Value: 1},
				Index:  &ast.Identifier{Value: "str"},
			},
			0, nil, true,
		},
		{
			"union via binary pipe expression",
			&ast.BinaryExpression{
				Left:     &ast.Identifier{Value: "str"},
				Operator: token.Token{Type: token.PIPE},
				Right:    &ast.Identifier{Value: "int"},
			},
			Union, nil, false,
		},
		{
			"binary expression with non-pipe operator errors",
			&ast.BinaryExpression{
				Left:     &ast.Identifier{Value: "str"},
				Operator: token.Token{Type: token.PLUS, Literal: "+"},
				Right:    &ast.Identifier{Value: "int"},
			},
			0, nil, true,
		},
		{
			// Coverage: exercises the default case in resolveType for unsupported expression types.
			"unsupported expression type errors",
			&ast.IntegerLiteral{Value: 42},
			0, nil, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ, err := ResolveTypeExpr(tt.expr)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if typ.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", typ.Kind, tt.kind)
			}
			if tt.expType != nil && !typ.Equals(*tt.expType) {
				t.Errorf("Type = %s, want %s", typ.String(), tt.expType.String())
			}
		})
	}
}

// TestResolveTypeExprInlineSchema verifies that inline schema expressions
// are resolved and registered with a synthetic name.
func TestResolveTypeExprInlineSchema(t *testing.T) {
	expr := &ast.BlockExpression{
		Kind: token.BlockSchema,
		Assignments: []*ast.Assignment{
			{
				Name:  "host",
				Value: &ast.Identifier{Value: "str"},
			},
		},
	}

	typ, err := ResolveTypeExpr(expr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ.Kind != BlockRef {
		t.Errorf("Kind = %v, want BlockRef", typ.Kind)
	}
	if typ.BlockKind != token.BlockSchema {
		t.Errorf("BlockKind = %v, want BlockSchema", typ.BlockKind)
	}
	if typ.SchemaName == "" {
		t.Error("expected non-empty SchemaName for inline schema")
	}

	// The registered schema should be accessible.
	schema, ok := GetSchema(typ.SchemaName)
	if !ok {
		t.Fatal("expected inline schema to be registered")
	}
	if _, hasHost := schema.Fields["host"]; !hasHost {
		t.Error("expected inline schema to have field 'host'")
	}
}

// TestNullStrippingInUnion verifies that null is correctly stripped from
// unions to determine optionality.
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
			"str | null becomes optional str",
			"schema test_strip {\n  @suppress(\"duplicate-block\")\n  field = str | null\n}",
			false, BlockRef, typePtr(Str()), 0,
		},
		{
			"str | model | null becomes optional union",
			"schema test_strip2 {\n  @suppress(\"duplicate-block\")\n  field = str | model | null\n}",
			false, Union, nil, 2,
		},
		{
			"str (no null) is required",
			"schema test_strip3 {\n  @suppress(\"duplicate-block\")\n  field = str\n}",
			true, BlockRef, typePtr(Str()), 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				t.Fatalf("parse errors: %v", p.Errors())
			}
			block := program.Statements[0].(*ast.BlockStatement)
			assign := block.Assignments[0]
			fs, err := ResolveFieldSchema(assign)
			if err != nil {
				t.Fatalf("ResolveFieldSchema error: %v", err)
			}
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

package types

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
	"github.com/thakee/orca/compiler/token"
)

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
		{"model", "model", 3},
		{"agent", "agent", 4},
		{"tool", "tool", 4},
		{"task", "task", 2},
		{"knowledge", "knowledge", 2},
		{"workflow", "workflow", 2},
		{"trigger", "trigger", 2},
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
		name      string
		block     string
		field     string
		kind      TypeKind        // expected Kind
		bk        token.BlockKind // expected BlockKind (for BlockRef)
		checkBK   bool            // whether to check BlockKind
		required  bool
	}{
		{"model.provider", "model", "provider", BlockRef, token.BlockStr, true, true},
		{"model.model_name", "model", "model_name", Union, 0, false, true},
		{"model.temperature", "model", "temperature", BlockRef, token.BlockFloat, true, false},
		{"agent.model", "agent", "model", Union, 0, false, true},
		{"agent.persona", "agent", "persona", BlockRef, token.BlockStr, true, true},
		{"agent.tools", "agent", "tools", List, 0, false, false},
		{"tool.name", "tool", "name", BlockRef, token.BlockStr, true, true},
		{"tool.desc", "tool", "desc", BlockRef, token.BlockStr, true, false},
		{"task.agent", "task", "agent", BlockRef, token.BlockAgent, true, true},
		{"task.prompt", "task", "prompt", BlockRef, token.BlockStr, true, true},
		{"knowledge.name", "knowledge", "name", BlockRef, token.BlockStr, true, true},
		{"workflow.name", "workflow", "name", BlockRef, token.BlockStr, true, false},
		{"trigger.name", "trigger", "name", BlockRef, token.BlockStr, true, false},
		{"input.type", "input", "type", BlockRef, token.BlockSchema, true, true},
		{"input.desc", "input", "desc", BlockRef, token.BlockStr, true, false},
		{"input.default", "input", "default", BlockRef, token.BlockAny, true, false},
		{"input.sensitive", "input", "sensitive", BlockRef, token.BlockBool, true, false},
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
			if tt.checkBK && field.Type.BlockKind != tt.bk {
				t.Errorf("BlockKind = %v, want %v", field.Type.BlockKind, tt.bk)
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
	if field.Type.Members[0].BlockKind != token.BlockStr {
		t.Errorf("first member BlockKind = %v, want %v", field.Type.Members[0].BlockKind, token.BlockStr)
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
		name      string
		input     string
		blockKind token.BlockKind
		schema    string // expected SchemaName for user schemas
		wantErr   bool
	}{
		{"primitive str", "str", token.BlockStr, "", false},
		{"primitive int", "int", token.BlockInt, "", false},
		{"primitive float", "float", token.BlockFloat, "", false},
		{"primitive bool", "bool", token.BlockBool, "", false},
		{"primitive any", "any", token.BlockAny, "", false},
		{"block type model", "model", token.BlockModel, "", false},
		{"block type agent", "agent", token.BlockAgent, "", false},
		{"block type schema", "schema", token.BlockSchema, "", false},
		{"user schema name", "vpc_data_t", token.BlockSchema, "vpc_data_t", false},
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
			if typ.BlockKind != tt.blockKind {
				t.Errorf("BlockKind = %v, want %v", typ.BlockKind, tt.blockKind)
			}
			if tt.schema != "" && typ.SchemaName != tt.schema {
				t.Errorf("SchemaName = %q, want %q", typ.SchemaName, tt.schema)
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
		bk      token.BlockKind
		wantErr bool
	}{
		{
			"null literal resolves to null type",
			&ast.NullLiteral{},
			BlockRef, token.BlockNull, false,
		},
		{
			"identifier str resolves to str",
			&ast.Identifier{Value: "str"},
			BlockRef, token.BlockStr, false,
		},
		{
			"identifier model resolves to model ref",
			&ast.Identifier{Value: "model"},
			BlockRef, token.BlockModel, false,
		},
		{
			"subscription list[str] resolves to list type",
			&ast.Subscription{
				Object: &ast.Identifier{Value: "list"},
				Index:  &ast.Identifier{Value: "str"},
			},
			List, 0, false,
		},
		{
			"subscription map[int] resolves to map type",
			&ast.Subscription{
				Object: &ast.Identifier{Value: "map"},
				Index:  &ast.Identifier{Value: "int"},
			},
			Map, 0, false,
		},
		{
			"unsupported parameterized type errors",
			&ast.Subscription{
				Object: &ast.Identifier{Value: "set"},
				Index:  &ast.Identifier{Value: "str"},
			},
			0, 0, true,
		},
		{
			"subscription with non-identifier base errors",
			&ast.Subscription{
				Object: &ast.IntegerLiteral{Value: 1},
				Index:  &ast.Identifier{Value: "str"},
			},
			0, 0, true,
		},
		{
			"union via binary pipe expression",
			&ast.BinaryExpression{
				Left:     &ast.Identifier{Value: "str"},
				Operator: token.Token{Type: token.PIPE},
				Right:    &ast.Identifier{Value: "int"},
			},
			Union, 0, false,
		},
		{
			"binary expression with non-pipe operator errors",
			&ast.BinaryExpression{
				Left:     &ast.Identifier{Value: "str"},
				Operator: token.Token{Type: token.PLUS, Literal: "+"},
				Right:    &ast.Identifier{Value: "int"},
			},
			0, 0, true,
		},
		{
			// Coverage: exercises the default case in resolveType for unsupported expression types.
			"unsupported expression type errors",
			&ast.IntegerLiteral{Value: 42},
			0, 0, true,
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
			if tt.bk != 0 && typ.BlockKind != tt.bk {
				t.Errorf("BlockKind = %v, want %v", typ.BlockKind, tt.bk)
			}
		})
	}
}

// TestResolveTypeExprInlineSchema verifies that inline schema expressions
// are resolved and registered with a synthetic name.
func TestResolveTypeExprInlineSchema(t *testing.T) {
	expr := &ast.SchemaExpression{
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
		resultBK    token.BlockKind
		checkBK     bool
		memberCount int
	}{
		{
			"str | null becomes optional str",
			"schema test_strip {\n  @suppress(\"duplicate-block\")\n  field = str | null\n}",
			false, BlockRef, token.BlockStr, true, 0,
		},
		{
			"str | model | null becomes optional union",
			"schema test_strip2 {\n  @suppress(\"duplicate-block\")\n  field = str | model | null\n}",
			false, Union, 0, false, 2,
		},
		{
			"str (no null) is required",
			"schema test_strip3 {\n  @suppress(\"duplicate-block\")\n  field = str\n}",
			true, BlockRef, token.BlockStr, true, 0,
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
			if tt.checkBK && fs.Type.BlockKind != tt.resultBK {
				t.Errorf("BlockKind = %v, want %v", fs.Type.BlockKind, tt.resultBK)
			}
			if tt.memberCount > 0 && len(fs.Type.Members) != tt.memberCount {
				t.Errorf("Members = %d, want %d", len(fs.Type.Members), tt.memberCount)
			}
		})
	}
}

package types

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
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
		{"agent", "agent", 3},
		{"tool", "tool", 2},
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
		name     string
		block    string
		field    string
		kind     TypeKind  // expected Kind
		bt       BlockKind // expected BlockType (for BlockRef)
		required bool
	}{
		{"model.provider", "model", "provider", BlockRef, "str", true},
		{"model.model_name", "model", "model_name", Union, "", true},
		{"model.temperature", "model", "temperature", BlockRef, "float", false},
		{"agent.model", "agent", "model", Union, "", true},
		{"agent.persona", "agent", "persona", BlockRef, "str", true},
		{"agent.tools", "agent", "tools", List, "", false},
		{"tool.name", "tool", "name", BlockRef, "str", true},
		{"tool.desc", "tool", "desc", BlockRef, "str", false},
		{"task.agent", "task", "agent", BlockRef, BlockAgent, true},
		{"task.prompt", "task", "prompt", BlockRef, "str", true},
		{"knowledge.name", "knowledge", "name", BlockRef, "str", true},
		{"workflow.name", "workflow", "name", BlockRef, "str", false},
		{"trigger.name", "trigger", "name", BlockRef, "str", false},
		{"input.type", "input", "type", BlockRef, BlockSchemaKind, true},
		{"input.desc", "input", "desc", BlockRef, "str", false},
		{"input.default", "input", "default", BlockRef, "any", false},
		{"input.sensitive", "input", "sensitive", BlockRef, "bool", false},
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
			if tt.bt != "" && field.Type.BlockType != tt.bt {
				t.Errorf("BlockType = %q, want %q", field.Type.BlockType, tt.bt)
			}
			if field.Required != tt.required {
				t.Errorf("Required = %v, want %v", field.Required, tt.required)
			}
		})
	}
}

// TestBlockKindFromNameSchema verifies that schema blocks have a BlockKind.
func TestBlockKindFromNameSchema(t *testing.T) {
	kind, ok := BlockKindFromName("schema")
	if !ok {
		t.Fatal("expected BlockKindFromName(\"schema\") to return true")
	}
	if kind != BlockSchemaKind {
		t.Errorf("kind = %v, want %v", kind, BlockSchemaKind)
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
	if field.Type.Members[0].BlockType != "str" {
		t.Errorf("first member BlockType = %q, want %q", field.Type.Members[0].BlockType, "str")
	}
	if field.Type.Members[1].BlockType != BlockModel {
		t.Errorf("second member BlockType = %q, want %q", field.Type.Members[1].BlockType, BlockModel)
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
	if field.Type.ElementType.BlockType != BlockTool {
		t.Errorf("ElementType.BlockType = %v, want %v", field.Type.ElementType.BlockType, BlockTool)
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
		blockType BlockKind
		wantErr   bool
	}{
		{"primitive str", "str", "str", false},
		{"primitive int", "int", "int", false},
		{"primitive float", "float", "float", false},
		{"primitive bool", "bool", "bool", false},
		{"primitive any", "any", "any", false},
		{"block type model", "model", BlockModel, false},
		{"block type agent", "agent", BlockAgent, false},
		{"block type schema", "schema", BlockSchemaKind, false},
		{"user schema name", "vpc_data_t", "vpc_data_t", false},
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
			if typ.BlockType != tt.blockType {
				t.Errorf("BlockType = %q, want %q", typ.BlockType, tt.blockType)
			}
		})
	}
}

// TestNullStrippingInUnion verifies that null is correctly stripped from
// unions to determine optionality.
func TestNullStrippingInUnion(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		required     bool
		resultKind   TypeKind
		resultBT     BlockKind
		memberCount  int
	}{
		{
			"str | null becomes optional str",
			"schema test_strip {\n  @suppress(\"duplicate-block\")\n  field = str | null\n}",
			false, BlockRef, "str", 0,
		},
		{
			"str | model | null becomes optional union",
			"schema test_strip2 {\n  @suppress(\"duplicate-block\")\n  field = str | model | null\n}",
			false, Union, "", 2,
		},
		{
			"str (no null) is required",
			"schema test_strip3 {\n  @suppress(\"duplicate-block\")\n  field = str\n}",
			true, BlockRef, "str", 0,
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
			if tt.resultBT != "" && fs.Type.BlockType != tt.resultBT {
				t.Errorf("BlockType = %q, want %q", fs.Type.BlockType, tt.resultBT)
			}
			if tt.memberCount > 0 && len(fs.Type.Members) != tt.memberCount {
				t.Errorf("Members = %d, want %d", len(fs.Type.Members), tt.memberCount)
			}
		})
	}
}

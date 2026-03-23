package types

import "testing"

// TestLoadSchemas verifies that the embedded block_schemas.oc file
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
		name         string
		blockType    string
		fieldName    string
		expectedKind TypeKind
		required     bool
	}{
		{"model.provider is string", "model", "provider", String, true},
		{"model.model_name is union", "model", "model_name", Union, false},
		{"model.temperature is float", "model", "temperature", Float, false},
		{"agent.model is union", "agent", "model", Union, true},
		{"agent.persona is string", "agent", "persona", String, true},
		{"agent.tools is list", "agent", "tools", List, false},
		{"tool.name is string", "tool", "name", String, true},
		{"tool.desc is string", "tool", "desc", String, false},
		{"task.agent is block ref", "task", "agent", BlockRef, true},
		{"task.prompt is string", "task", "prompt", String, true},
		{"knowledge.name is string", "knowledge", "name", String, true},
		{"workflow.name is string", "workflow", "name", String, false},
		{"trigger.name is string", "trigger", "name", String, false},
		{"input.type is schema ref", "input", "type", BlockRef, true},
		{"input.desc is string", "input", "desc", String, false},
		{"input.default is any", "input", "default", Any, false},
		{"input.sensitive is bool", "input", "sensitive", Bool, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := schemas[tt.blockType].Fields[tt.fieldName]
			if !ok {
				t.Fatalf("field %s.%s not found", tt.blockType, tt.fieldName)
			}
			if field.Type.Kind != tt.expectedKind {
				t.Errorf("Kind = %v, want %v", field.Type.Kind, tt.expectedKind)
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

	// model.model_name should be str | model (null stripped, Required=false).
	field := schemas["model"].Fields["model_name"]
	if field.Type.Kind != Union {
		t.Fatalf("expected Union, got %v", field.Type.Kind)
	}
	if len(field.Type.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(field.Type.Members))
	}
	if field.Type.Members[0].Kind != String {
		t.Errorf("first member Kind = %v, want String", field.Type.Members[0].Kind)
	}
	if field.Type.Members[1].Kind != BlockRef {
		t.Errorf("second member Kind = %v, want BlockRef", field.Type.Members[1].Kind)
	}
	if field.Type.Members[1].BlockType != BlockModel {
		t.Errorf("second member BlockType = %v, want %v", field.Type.Members[1].BlockType, BlockModel)
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

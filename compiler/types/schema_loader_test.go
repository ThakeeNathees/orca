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
		{"model", "model", 3},
		{"agent", "agent", 3},
		{"tool", "tool", 6},
		{"task", "task", 2},
		{"knowledge", "knowledge", 2},
		{"workflow", "workflow", 2},
		{"trigger", "trigger", 3},
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
		{"agent.tools is list", "agent", "tools", List, false},
		{"agent.prompt is string", "agent", "prompt", String, true},
		{"tool.type is string", "tool", "type", String, true},
		{"task.agent is block ref", "task", "agent", BlockRef, true},
		{"task.prompt is string", "task", "prompt", String, true},
		{"workflow.flow is any", "workflow", "flow", Any, true},
		{"trigger.workflow is block ref", "trigger", "workflow", BlockRef, false},
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

// TestLoadSchemasUnionMembers verifies that union types contain the
// correct member types.
func TestLoadSchemasUnionMembers(t *testing.T) {
	schemas, err := loadSchemas()
	if err != nil {
		t.Fatalf("loadSchemas() error: %v", err)
	}

	// model.model_name should be str | model
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

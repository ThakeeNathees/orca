package types

import "testing"

// TestGetBlockSchema verifies that block schemas are correctly returned.
func TestGetBlockSchema(t *testing.T) {
	tests := []struct {
		name      string
		blockType string
		ok        bool
		numFields int
	}{
		{"model schema exists", "model", true, 3},
		{"agent schema exists", "agent", true, 3},
		{"tool schema exists", "tool", true, 2},
		{"task schema exists", "task", true, 2},
		{"knowledge schema exists", "knowledge", true, 2},
		{"workflow schema exists", "workflow", true, 2},
		{"trigger schema exists", "trigger", true, 2},
		{"input schema exists", "input", true, 4},
		{"primitive str", "str", true, 0},
		{"unknown block", "foobar", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, ok := GetBlockSchema(tt.blockType)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && len(schema.Fields) != tt.numFields {
				t.Errorf("num fields = %d, want %d", len(schema.Fields), tt.numFields)
			}
		})
	}
}

// TestGetFieldSchema verifies field lookups within block schemas.
func TestGetFieldSchema(t *testing.T) {
	tests := []struct {
		name         string
		blockType    string
		fieldName    string
		ok           bool
		expectedKind TypeKind
		required     bool
	}{
		{"model provider", "model", "provider", true, String, true},
		{"model model_name", "model", "model_name", true, Union, false},
		{"model temperature", "model", "temperature", true, Float, false},
		{"agent model union", "agent", "model", true, Union, true},
		{"agent persona", "agent", "persona", true, String, true},
		{"agent tools list", "agent", "tools", true, List, false},
		{"unknown field", "model", "nonexistent", false, Any, false},
		{"unknown block", "foobar", "anything", false, Any, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := GetFieldSchema(tt.blockType, tt.fieldName)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok {
				if field.Type.Kind != tt.expectedKind {
					t.Errorf("Kind = %v, want %v", field.Type.Kind, tt.expectedKind)
				}
				if field.Required != tt.required {
					t.Errorf("Required = %v, want %v", field.Required, tt.required)
				}
			}
		})
	}
}

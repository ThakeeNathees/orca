package types

import (
	"testing"

	"github.com/thakee/orca/compiler/token"
)

// TestGetBlockSchema verifies that block schemas are correctly returned.
func TestGetBlockSchema(t *testing.T) {
	tests := []struct {
		name      string
		blockKind token.BlockKind
		ok        bool
		numFields int
	}{
		{"model schema exists", token.BlockModel, true, 3},
		{"agent schema exists", token.BlockAgent, true, 4},
		{"tool schema exists", token.BlockTool, true, 4},
		{"task schema exists", token.BlockTask, true, 2},
		{"knowledge schema exists", token.BlockKnowledge, true, 2},
		{"workflow schema exists", token.BlockWorkflow, true, 2},
		{"trigger schema exists", token.BlockTrigger, true, 2},
		{"input schema exists", token.BlockInput, true, 4},
		{"primitive str", token.BlockStr, true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, ok := GetBlockSchema(tt.blockKind)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && len(schema.Fields) != tt.numFields {
				t.Errorf("num fields = %d, want %d", len(schema.Fields), tt.numFields)
			}
		})
	}
}

// TestGetSchemaByName verifies string-based schema lookups.
func TestGetSchemaByName(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		ok     bool
	}{
		{"builtin model", "model", true},
		{"builtin str", "str", true},
		{"unknown", "foobar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := GetSchema(tt.schema)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
		})
	}
}

// TestGetSchemaUserDefined verifies lookup of a registered user schema.
func TestGetSchemaUserDefined(t *testing.T) {
	RegisterSchema("test_user_schema", BlockSchema{
		Fields: map[string]FieldSchema{
			"name": {Type: TypeOf(token.BlockStr), Required: true},
		},
	})
	schema, ok := GetSchema("test_user_schema")
	if !ok {
		t.Fatal("expected registered user schema to be found")
	}
	if len(schema.Fields) != 1 {
		t.Errorf("num fields = %d, want 1", len(schema.Fields))
	}
}

// TestLookupBlockSchemaBuiltin verifies LookupBlockSchema for built-in block types.
func TestLookupBlockSchemaBuiltin(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		ok   bool
	}{
		{"model type", NewBlockRefType(token.BlockModel), true},
		{"agent type", NewBlockRefType(token.BlockAgent), true},
		{"str type", TypeOf(token.BlockStr), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := LookupBlockSchema(tt.typ)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
		})
	}
}

// TestLookupBlockSchemaUserSchema verifies LookupBlockSchema dispatches
// to the schema name for user-defined schemas.
func TestLookupBlockSchemaUserSchema(t *testing.T) {
	RegisterSchema("test_lookup_schema", BlockSchema{
		Fields: map[string]FieldSchema{
			"host": {Type: TypeOf(token.BlockStr), Required: true},
			"port": {Type: TypeOf(token.BlockInt), Required: false},
		},
	})

	typ := SchemaTypeOf("test_lookup_schema")
	schema, ok := LookupBlockSchema(typ)
	if !ok {
		t.Fatal("expected user schema to be found via LookupBlockSchema")
	}
	if len(schema.Fields) != 2 {
		t.Errorf("num fields = %d, want 2", len(schema.Fields))
	}
}

// TestLookupBlockSchemaUnknown verifies LookupBlockSchema returns false for
// unknown schema names.
func TestLookupBlockSchemaUnknown(t *testing.T) {
	typ := SchemaTypeOf("nonexistent_schema")
	_, ok := LookupBlockSchema(typ)
	if ok {
		t.Error("expected unknown user schema to return false")
	}
}

// TestLookupFieldSchemaBuiltin verifies LookupFieldSchema for built-in types.
func TestLookupFieldSchemaBuiltin(t *testing.T) {
	typ := NewBlockRefType(token.BlockModel)
	field, ok := LookupFieldSchema(typ, "provider")
	if !ok {
		t.Fatal("expected model.provider to be found")
	}
	if field.Type.BlockKind != token.BlockStr {
		t.Errorf("BlockKind = %v, want %v", field.Type.BlockKind, token.BlockStr)
	}
}

// TestLookupFieldSchemaUserSchema verifies LookupFieldSchema resolves fields
// through user-defined schemas.
func TestLookupFieldSchemaUserSchema(t *testing.T) {
	RegisterSchema("test_field_lookup", BlockSchema{
		Fields: map[string]FieldSchema{
			"region": {Type: TypeOf(token.BlockStr), Required: true},
		},
	})

	typ := SchemaTypeOf("test_field_lookup")
	field, ok := LookupFieldSchema(typ, "region")
	if !ok {
		t.Fatal("expected field 'region' to be found")
	}
	if field.Type.BlockKind != token.BlockStr {
		t.Errorf("BlockKind = %v, want %v", field.Type.BlockKind, token.BlockStr)
	}

	// Unknown field returns false.
	_, ok = LookupFieldSchema(typ, "nonexistent")
	if ok {
		t.Error("expected unknown field to return false")
	}
}

// TestBuiltinSchemaNames verifies that BuiltinSchemaNames returns a non-empty
// list of schema names loaded from builtins.oc.
func TestBuiltinSchemaNames(t *testing.T) {
	names := BuiltinSchemaNames()
	if len(names) == 0 {
		t.Fatal("expected BuiltinSchemaNames to return at least one name")
	}

	// Verify well-known builtins are present.
	tests := []struct {
		name     string
		expected string
	}{
		{"model is builtin", "model"},
		{"agent is builtin", "agent"},
		{"str is builtin", "str"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := false
			for _, n := range names {
				if n == tt.expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %q to be in BuiltinSchemaNames", tt.expected)
			}
		})
	}
}

// TestRegisterSchemaAndGetSchema verifies that RegisterSchema makes a schema
// available via GetSchema.
func TestRegisterSchemaAndGetSchema(t *testing.T) {
	tests := []struct {
		name      string
		schemaKey string
		fields    map[string]FieldSchema
	}{
		{
			"register single-field schema",
			"test_reg_schema_1",
			map[string]FieldSchema{
				"url": {Type: TypeOf(token.BlockStr), Required: true},
			},
		},
		{
			"register multi-field schema",
			"test_reg_schema_2",
			map[string]FieldSchema{
				"host": {Type: TypeOf(token.BlockStr), Required: true},
				"port": {Type: TypeOf(token.BlockInt), Required: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterSchema(tt.schemaKey, BlockSchema{Fields: tt.fields})
			schema, ok := GetSchema(tt.schemaKey)
			if !ok {
				t.Fatalf("expected schema %q to be found after registration", tt.schemaKey)
			}
			if len(schema.Fields) != len(tt.fields) {
				t.Errorf("num fields = %d, want %d", len(schema.Fields), len(tt.fields))
			}
		})
	}
}

// Coverage: exercises GetFieldSchema with an unknown block kind that has no schema.
func TestGetFieldSchemaUnknownBlock(t *testing.T) {
	tests := []struct {
		name      string
		blockKind token.BlockKind
		field     string
		ok        bool
	}{
		{"unknown block kind", token.BlockKind(999), "anything", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := GetFieldSchema(tt.blockKind, tt.field)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
		})
	}
}

// Coverage: exercises LookupFieldSchema when the block schema is not found.
func TestLookupFieldSchemaUnknownSchema(t *testing.T) {
	tests := []struct {
		name  string
		typ   Type
		field string
		ok    bool
	}{
		{"nonexistent user schema", SchemaTypeOf("totally_unknown_schema"), "field", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := LookupFieldSchema(tt.typ, tt.field)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
		})
	}
}

// TestGetFieldSchema verifies field lookups within block schemas.
func TestGetFieldSchema(t *testing.T) {
	tests := []struct {
		name      string
		blockKind token.BlockKind
		field     string
		ok        bool
		kind      TypeKind
		bk        token.BlockKind
		checkBK   bool
		required  bool
	}{
		{"model provider", token.BlockModel, "provider", true, BlockRef, token.BlockStr, true, true},
		{"model model_name", token.BlockModel, "model_name", true, Union, 0, false, true},
		{"model temperature", token.BlockModel, "temperature", true, BlockRef, token.BlockFloat, true, false},
		{"agent model union", token.BlockAgent, "model", true, Union, 0, false, true},
		{"agent persona", token.BlockAgent, "persona", true, BlockRef, token.BlockStr, true, true},
		{"agent tools list", token.BlockAgent, "tools", true, List, 0, false, false},
		{"unknown field", token.BlockModel, "nonexistent", false, BlockRef, 0, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := GetFieldSchema(tt.blockKind, tt.field)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok {
				if field.Type.Kind != tt.kind {
					t.Errorf("Kind = %v, want %v", field.Type.Kind, tt.kind)
				}
				if tt.checkBK && field.Type.BlockKind != tt.bk {
					t.Errorf("BlockKind = %v, want %v", field.Type.BlockKind, tt.bk)
				}
				if field.Required != tt.required {
					t.Errorf("Required = %v, want %v", field.Required, tt.required)
				}
			}
		})
	}
}

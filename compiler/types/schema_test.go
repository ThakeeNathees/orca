package types

import (
	"testing"
)

// bootstrapSchemaPointers returns a map of block name -> *BlockSchema from the
// embedded bootstrap.oc. Pointers reference the slice backing BootstrapResult.Schemas.
func bootstrapSchemaPointers(t *testing.T) map[string]*BlockSchema {
	t.Helper()
	res := Bootstrap(testBootstrapSource)
	m := make(map[string]*BlockSchema)
	for i := range res.Schemas {
		s := &res.Schemas[i]
		m[s.BlockName] = s
	}
	return m
}

// bootstrapSymtab returns the symbol table produced alongside bootstrap schemas.
func bootstrapSymtab(t *testing.T) *SymbolTable {
	t.Helper()
	return Bootstrap(testBootstrapSource).Symtab
}

// fieldFromBootstrap returns a field schema from bootstrapped block definitions.
func fieldFromBootstrap(schemas map[string]*BlockSchema, blockName, field string) (FieldSchema, bool) {
	s, ok := schemas[blockName]
	if !ok {
		return FieldSchema{}, false
	}
	fs, ok := s.Fields[field]
	return fs, ok
}

// TestGetBlockSchema verifies that block schemas are present after bootstrap.
func TestGetBlockSchema(t *testing.T) {
	schemas := bootstrapSchemaPointers(t)

	tests := []struct {
		name      string
		blockName string
		ok        bool
		numFields int
	}{
		{"model schema exists", "model", true, 5},
		{"agent schema exists", "agent", true, 4},
		{"tool schema exists", "tool", true, 4},
		{"knowledge schema exists", "knowledge", true, 1},
		{"workflow schema exists", "workflow", true, 2},
		{"cron schema exists", "cron", true, 2},
		{"webhook schema exists", "webhook", true, 2},
		{"schema meta-schema exists (bootstrap schema schema {})", BlockKindSchema, true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, ok := schemas[tt.blockName]
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && len(s.Fields) != tt.numFields {
				t.Errorf("num fields = %d, want %d", len(s.Fields), tt.numFields)
			}
		})
	}
}

// TestGetSchemaByName verifies string-based schema lookups after bootstrap.
func TestGetSchemaByName(t *testing.T) {
	schemas := bootstrapSchemaPointers(t)

	tests := []struct {
		name   string
		schema string
		ok     bool
	}{
		{"builtin model", "model", true},
		{"builtin string", "string", true},
		{"unknown", "foobar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := schemas[tt.schema]
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
		})
	}
}

// TestGetSchemaUserDefined verifies a manually constructed BlockSchema holds fields
// and works with LookupFieldSchema when attached to a type.
func TestGetSchemaUserDefined(t *testing.T) {
	st := bootstrapSymtab(t)
	user := BlockSchema{
		BlockName: "test_user_schema",
		Fields: map[string]FieldSchema{
			"name": {Type: IdentType(0, "string", st), Required: true},
		},
	}
	typ := NewBlockRefType("test_user_schema", &user)
	field, ok := LookupFieldSchema(typ, "name")
	if !ok {
		t.Fatal("expected field 'name' on user schema")
	}
	if !field.Type.Equals(IdentType(0, "string", st)) {
		t.Errorf("Type = %s, want string", field.Type.String())
	}
	if len(user.Fields) != 1 {
		t.Errorf("num fields = %d, want 1", len(user.Fields))
	}
}

// TestLookupBlockSchemaBuiltin verifies LookupFieldSchema for built-in block types
// when the type carries a pointer to the bootstrapped BlockSchema.
func TestLookupBlockSchemaBuiltin(t *testing.T) {
	schemas := bootstrapSchemaPointers(t)

	tests := []struct {
		name   string
		typ    Type
		field  string
		wantOK bool
	}{
		{"model type", NewBlockRefType("model", schemas["model"]), "provider", true},
		{"agent type", NewBlockRefType("agent", schemas["agent"]), "persona", true},
		{"string type (no fields)", NewBlockRefType("string", schemas["string"]), "x", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := LookupFieldSchema(tt.typ, tt.field)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

// TestLookupBlockSchemaUserSchema verifies LookupFieldSchema resolves fields
// through user-defined schemas attached to the type.
func TestLookupBlockSchemaUserSchema(t *testing.T) {
	st := bootstrapSymtab(t)
	user := BlockSchema{
		BlockName: "test_lookup_schema",
		Fields: map[string]FieldSchema{
			"host": {Type: IdentType(0, "string", st), Required: true},
			"port": {Type: IdentType(0, "number", st), Required: false},
		},
	}

	typ := NewBlockRefType("test_lookup_schema", &user)
	field, ok := LookupFieldSchema(typ, "host")
	if !ok {
		t.Fatal("expected user schema field 'host' to be found via LookupFieldSchema")
	}
	if !field.Type.Equals(IdentType(0, "string", st)) {
		t.Errorf("Type = %s, want string", field.Type.String())
	}
	if len(user.Fields) != 2 {
		t.Errorf("num fields = %d, want 2", len(user.Fields))
	}
}

// TestLookupBlockSchemaUnknown verifies LookupFieldSchema returns false when
// the type has no resolved BlockSchema.
func TestLookupBlockSchemaUnknown(t *testing.T) {
	typ := NewBlockRefType("nonexistent_schema", nil)
	_, ok := LookupFieldSchema(typ, "field")
	if ok {
		t.Error("expected unknown schema (nil Block) to return false")
	}
}

// TestLookupFieldSchemaBuiltin verifies LookupFieldSchema for built-in types.
func TestLookupFieldSchemaBuiltin(t *testing.T) {
	schemas := bootstrapSchemaPointers(t)
	typ := NewBlockRefType("model", schemas["model"])
	field, ok := LookupFieldSchema(typ, "provider")
	if !ok {
		t.Fatal("expected model.provider to be found")
	}
	if field.Type.String() != "string" || field.Type.Kind != BlockRef {
		t.Errorf("Type = %s (Kind=%v), want BlockRef %s", field.Type.String(), field.Type.Kind, "string")
	}
}

// TestLookupFieldSchemaUserSchema verifies LookupFieldSchema resolves fields
// through user-defined schemas.
func TestLookupFieldSchemaUserSchema(t *testing.T) {
	st := bootstrapSymtab(t)
	user := BlockSchema{
		BlockName: "test_field_lookup",
		Fields: map[string]FieldSchema{
			"region": {Type: IdentType(0, "string", st), Required: true},
		},
	}

	typ := NewBlockRefType("test_field_lookup", &user)
	field, ok := LookupFieldSchema(typ, "region")
	if !ok {
		t.Fatal("expected field 'region' to be found")
	}
	if !field.Type.Equals(IdentType(0, "string", st)) {
		t.Errorf("Type = %s, want string", field.Type.String())
	}

	// Unknown field returns false.
	_, ok = LookupFieldSchema(typ, "nonexistent")
	if ok {
		t.Error("expected unknown field to return false")
	}
}

// TestBuiltinSchemaNames verifies bootstrap yields a non-empty set of block names.
func TestBuiltinSchemaNames(t *testing.T) {
	schemas := bootstrapSchemaPointers(t)
	if len(schemas) == 0 {
		t.Fatal("expected bootstrap to register at least one schema")
	}

	tests := []struct {
		name     string
		expected string
	}{
		{"model is builtin", "model"},
		{"agent is builtin", "agent"},
		{"string is builtin", "string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := schemas[tt.expected]; !ok {
				t.Errorf("expected %q to be present after bootstrap", tt.expected)
			}
		})
	}
}

// TestRegisterSchemaAndGetSchema verifies a locally built BlockSchema retains
// its fields and works with LookupFieldSchema (no global registry).
func TestRegisterSchemaAndGetSchema(t *testing.T) {
	st := bootstrapSymtab(t)
	tests := []struct {
		name      string
		schemaKey string
		fields    map[string]FieldSchema
	}{
		{
			"register single-field schema",
			"test_reg_schema_1",
			map[string]FieldSchema{
				"url": {Type: IdentType(0, "string", st), Required: true},
			},
		},
		{
			"register multi-field schema",
			"test_reg_schema_2",
			map[string]FieldSchema{
				"host": {Type: IdentType(0, "string", st), Required: true},
				"port": {Type: IdentType(0, "number", st), Required: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := BlockSchema{
				BlockName: tt.schemaKey,
				Fields:    tt.fields,
			}
			typ := NewBlockRefType(tt.schemaKey, &bs)
			for fname := range tt.fields {
				f, ok := LookupFieldSchema(typ, fname)
				if !ok {
					t.Fatalf("expected field %q to be found", fname)
				}
				if !f.Type.Equals(tt.fields[fname].Type) {
					t.Errorf("field %q type = %s, want %s", fname, f.Type.String(), tt.fields[fname].Type.String())
				}
			}
			if len(bs.Fields) != len(tt.fields) {
				t.Errorf("num fields = %d, want %d", len(bs.Fields), len(tt.fields))
			}
		})
	}
}

// Coverage: field lookup for an unknown block name in bootstrap data.
func TestGetFieldSchemaUnknownBlock(t *testing.T) {
	schemas := bootstrapSchemaPointers(t)
	tests := []struct {
		name      string
		blockName string
		field     string
		ok        bool
	}{
		{"unknown block name", "unknown", "anything", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := fieldFromBootstrap(schemas, tt.blockName, tt.field)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
		})
	}
}

// Coverage: LookupFieldSchema when the type has no BlockSchema pointer.
func TestLookupFieldSchemaUnknownSchema(t *testing.T) {
	tests := []struct {
		name  string
		typ   Type
		field string
		ok    bool
	}{
		{"nonexistent user schema", NewBlockRefType("totally_unknown_schema", nil), "field", false},
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

// TestGetFieldSchema verifies field lookups within block schemas loaded from bootstrap.
func TestGetFieldSchema(t *testing.T) {
	schemas := bootstrapSchemaPointers(t)

	tests := []struct {
		name      string
		blockName string
		field     string
		ok        bool
		kind      TypeKind
		wantStr   string
		required  bool
	}{
		{"model provider", "model", "provider", true, BlockRef, "string", true},
		{"model model_name", "model", "model_name", true, BlockRef, "string", true},
		{"model temperature", "model", "temperature", true, Union, "number | null", false},
		{"agent model union", "agent", "model", true, Union, "string | model", true},
		{"agent persona", "agent", "persona", true, BlockRef, "string", true},
		{"agent tools list", "agent", "tools", true, Union, "list[tool] | null", false},
		{"unknown field", "model", "nonexistent", false, BlockRef, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := fieldFromBootstrap(schemas, tt.blockName, tt.field)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok {
				if field.Type.Kind != tt.kind {
					t.Errorf("Kind = %v, want %v", field.Type.Kind, tt.kind)
				}
				if got := field.Type.String(); got != tt.wantStr {
					t.Errorf("Type.String() = %q, want %q", got, tt.wantStr)
				}
				if field.Required != tt.required {
					t.Errorf("Required = %v, want %v", field.Required, tt.required)
				}
			}
		})
	}
}

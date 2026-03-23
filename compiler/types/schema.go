// Package types provides block field schemas that define the expected
// types for each field within each block type. The analyzer will use
// these schemas to validate assignments in .oc files.
package types

// FieldSchema describes the expected type and constraints for a single
// field within a block.
type FieldSchema struct {
	Type        Type   // the expected type of this field's value
	Required    bool   // whether this field must be present in the block
	Description string // optional human-readable description of the field
}

// BlockSchema defines the set of valid fields for a block type,
// mapping field names to their schemas.
type BlockSchema struct {
	Fields map[string]FieldSchema
}

// blockSchemas maps block type names to their field schemas.
// Populated at init time by loading the embedded block_schemas.oc file.
// Used by the analyzer to validate that assignments within blocks
// have the correct types and that required fields are present.
var blockSchemas map[string]BlockSchema

// GetBlockSchema returns the schema for the given block type name.
// Returns the schema and true if found, or an empty schema and false
// if the block type has no schema defined.
func GetBlockSchema(blockType string) (BlockSchema, bool) {
	schema, ok := blockSchemas[blockType]
	return schema, ok
}

// BuiltinSchemaNames returns the names of all schemas loaded from
// block_schemas.oc. Used by the analyzer to pre-populate the symbol
// table so that built-in type names (str, int, model, etc.) are
// recognized as valid references.
func BuiltinSchemaNames() []string {
	names := make([]string, 0, len(blockSchemas))
	for name := range blockSchemas {
		names = append(names, name)
	}
	return names
}

// GetFieldSchema returns the field schema for a specific field within
// a block type. Returns the field schema and true if found.
func GetFieldSchema(blockType, fieldName string) (FieldSchema, bool) {
	schema, ok := blockSchemas[blockType]
	if !ok {
		return FieldSchema{}, false
	}
	field, ok := schema.Fields[fieldName]
	return field, ok
}

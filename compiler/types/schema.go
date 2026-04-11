// Package types provides block field schemas that define the expected
// types for each field within each block type. The analyzer will use
// these schemas to validate assignments in .oc files.
package types

import "github.com/thakee/orca/compiler/ast"

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

	// Block name is the given name of the block, if it's an anonymous block
	// we use the generated anon name.
	//   foo bar {...}   -> BlockName is "bar"
	//   bar = foo {...} -> BlockName is "__anon_<id>"
	BlockName string

	Ast         *ast.BlockBody
	Annotations []*ast.Annotation
	Fields      map[string]FieldSchema

	// The schema of the block.
	// +------------------+------------------+
	// | Block            | Schema           |
	// +------------------+------------------+
	// | foo bar {}       | schema foo {}    |
	// | agent writer {}  | schema agent {}  |
	// | schema string {} | schema schema {} |
	// | schema schema {} | schema schema {} |  <-- schema's schema is the schema itself.
	// +------------------+------------------+
	Schema *BlockSchema
}

// NewBlockSchema builds a BlockSchema from a schema block's assignments.
// Each assignment is resolved using ResolveFieldSchema, producing field
// types, required flags, and descriptions from annotations.
func NewBlockSchema(
	annotations []*ast.Annotation,
	blockName string,
	block *ast.BlockBody,
	symtab *SymbolTable,
) BlockSchema {
	fields := make(map[string]FieldSchema)
	for _, assign := range block.Assignments {
		fs := NewFieldSchema(assign, symtab)
		fields[assign.Name] = fs
	}
	return BlockSchema{
		BlockName:   blockName,
		Ast:         block,
		Annotations: annotations,
		Fields:      fields,
	}
}

// NewFieldSchema extracts a FieldSchema from an assignment using the
// annotation-based format. The assignment value is the type expression
// (e.g. string, string | model | null). Annotations provide metadata:
// @desc("...") for descriptions. Required is inferred: if the type
// contains null in a union, the field is optional; otherwise required.
func NewFieldSchema(assign *ast.Assignment, symtab *SymbolTable) FieldSchema {
	typ := ExprTypeFromExpr(assign.Value, symtab)
	fs := FieldSchema{Required: true, Type: typ}

	// If the type is a union containing null, the field is optional.
	if typ.Kind == Union {
		for _, m := range typ.Members {
			if m.IsNull() {
				fs.Required = false
			}
		}
	}

	// Extract metadata from annotations.
	for _, ann := range assign.Annotations {
		switch ann.Name {
		case "desc":
			if len(ann.Arguments) == 1 {
				if strLit, ok := ann.Arguments[0].(*ast.StringLiteral); ok {
					fs.Description = strLit.Value
				}
			}
		}
	}

	return fs
}

// NewLambdaParamSchema creates a synthetic BlockSchema for a lambda parameter.
// The Ast.Kind is set to the param type name (e.g. "number") so that IdentType's
// depth chain resolves correctly: param "n" → kind "number" → schema number {}.
func NewLambdaParamSchema(paramName string, paramType Type) BlockSchema {
	// paramType from SchemaTypeFromExpr(depth=1) for "number" is:
	//   Type{BlockRef, BlockName: "number", Block: <schema number {}>}
	// We set Ast.Kind to the type name ("number") so the depth chain works:
	//   identType(1, "n") → Ast.Kind = "number" → identType(0, "number") → schema number {}
	return BlockSchema{
		BlockName: paramName,
		Ast:       &ast.BlockBody{Kind: paramType.BlockName},
		Schema:    paramType.Block,
	}
}

// IsEqualTo returns true if this BlockSchema is equal to the other.
// Equality is defined structurally: BlockName, fields, schema pointer.
// Note that FieldSchema equality is shallow (Type struct equality).
func (b *BlockSchema) IsEqualTo(other *BlockSchema) bool {
	if b == nil || other == nil {
		return b == other
	}

	// This is debatable.
	if b.BlockName != other.BlockName {
		return false
	}

	// Compare the number of fields
	if len(b.Fields) != len(other.Fields) {
		return false
	}

	// TODO: Implement IsEqualTo for FieldSchema.
	//
	// Compare field contents
	// for name, f := range b.Fields {
	// 	of, ok := other.Fields[name]
	// 	if !ok {
	// 		return false
	// 	}
	// 	// Compare FieldSchema by value (Type, Required, Description)
	// 	if f.IsEqualTo(&of) {
	// 		return false
	// 	}
	// }

	// Compare schema pointer, we can also use b.Schema.IsEqualTo(other.Schema).
	// but I afrid tha twe might run into infinit recursion, because schema's schema
	// is schema itself, will have to test this.
	if b.Schema != other.Schema {
		return false
	}

	// Just comparing BlockName, Fields, and Schema.
	// Optional: compare Ast and Annotations if needed.
	return true
}

// LookupFieldSchema returns the field schema for a named field within a Type,
// dispatching between built-in block schemas and user-defined schemas.
func LookupFieldSchema(t Type, fieldName string) (FieldSchema, bool) {
	if t.Kind != BlockRef || t.Block == nil {
		return FieldSchema{}, false
	}

	if schema, ok := t.Block.Fields[fieldName]; ok {
		return schema, true
	}
	return FieldSchema{}, false
}

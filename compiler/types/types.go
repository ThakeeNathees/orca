// Package types defines the Orca type system used for semantic analysis.
// Block types (model, agent, etc.) are represented as BlockRef with a BlockKind enum.
// Primitive types (str, int, float, bool, any, null) are schemas defined in
// builtins.oc and represented as SchemaTypeOf("str"), etc. Structural types
// (List, Map, Union) are separate kinds.
package types

import "github.com/thakee/orca/compiler/token"

// TypeKind classifies the broad category of a type.
type TypeKind int

const (
	// BlockRef represents a named type — block types (model, agent)
	// and schema types (built-in primitives and user-defined schemas).
	BlockRef TypeKind = iota
	// List represents an ordered collection of elements.
	List
	// Map represents a collection of key-value pairs.
	Map
	// Union represents a type that can be one of several member types.
	Union
)

// kindStrings maps each TypeKind to its human-readable name.
var kindStrings = map[TypeKind]string{
	BlockRef: "block_ref",
	List:     "list",
	Map:      "map",
	Union:    "union",
}

// BlockKind is a type alias for token.BlockKind, used throughout the types package.
type BlockKind = token.BlockKind

// String returns the human-readable name of this type kind.
func (k TypeKind) String() string {
	if s, ok := kindStrings[k]; ok {
		return s
	}
	return "unknown"
}

// Type represents a concrete type in the Orca type system.
// Block types (model, agent) use Kind=BlockRef with BlockKind set.
// Schema types (str, int, user schemas) use Kind=BlockRef with
// BlockKind=BlockSchema and SchemaName set.
// Structural types use Kind=List, Map, or Union.
type Type struct {
	Kind        TypeKind
	ElementType *Type           // non-nil for List types
	KeyType     *Type           // non-nil for Map types
	ValueType   *Type           // non-nil for Map types
	BlockKind   token.BlockKind // the block kind for BlockRef types
	SchemaName  string          // for schema types (BlockKind == BlockSchema): empty means "any schema", non-empty is a specific schema name (__anon_N for inline, user name for declared)
	Members     []Type          // non-nil for Union types — the set of acceptable types
}

// typeCache stores BlockRef types keyed by BlockKind, extended on demand by TypeOf.
var typeCache = make(map[token.BlockKind]Type)

// schemaTypeCache stores BlockRef types for schema types keyed by name.
var schemaTypeCache = make(map[string]Type)

// TypeOf returns the cached BlockRef type for a given BlockKind.
// Only valid for block kinds (model, agent, etc.), not primitives.
func TypeOf(kind token.BlockKind) Type {
	if t, ok := typeCache[kind]; ok {
		return t
	}
	t := Type{Kind: BlockRef, BlockKind: kind}
	typeCache[kind] = t
	return t
}

// CreateSchema returns the cached BlockRef type for a schema name.
// Used for both built-in primitives (str, int, etc.) and user-defined schemas.
func CreateSchema(name string) Type {
	if t, ok := schemaTypeCache[name]; ok {
		return t
	}
	t := Type{Kind: BlockRef, BlockKind: token.BlockSchema, SchemaName: name}
	schemaTypeCache[name] = t
	return t
}

// Primitive type accessors for built-in types defined in builtins.oc.
func Str() Type   { return CreateSchema("str") }
func Int() Type   { return CreateSchema("int") }
func Float() Type { return CreateSchema("float") }
func Bool() Type  { return CreateSchema("bool") }
func Any() Type   { return CreateSchema("any") }
func Null() Type  { return CreateSchema("null") }

// IsAny returns true if this type is the "any" type (matches everything).
func (t Type) IsAny() bool {
	return t.Kind == BlockRef && t.BlockKind == token.BlockSchema && t.SchemaName == "any"
}

// IsNull returns true if this type is the "null" type.
func (t Type) IsNull() bool {
	return t.Kind == BlockRef && t.BlockKind == token.BlockSchema && t.SchemaName == "null"
}

// NewListType creates a list type with the given element type.
func NewListType(element Type) Type {
	return Type{Kind: List, ElementType: &element}
}

// NewMapType creates a map type with the given key and value types.
func NewMapType(key, value Type) Type {
	return Type{Kind: Map, KeyType: &key, ValueType: &value}
}

// NewBlockRefType creates a block reference type with the given BlockKind.
func NewBlockRefType(kind token.BlockKind) Type {
	return Type{Kind: BlockRef, BlockKind: kind}
}

// NewLetType creates a type for a named let block. Uses BlockKind=BlockLet
// with a SchemaName so that member access can resolve fields through the
// per-instance schema registered for this let block.
func NewLetType(name string) Type {
	return Type{Kind: BlockRef, BlockKind: token.BlockLet, SchemaName: name}
}

// NewUnionType creates a union type that accepts any of the given member types.
func NewUnionType(members ...Type) Type {
	return Type{Kind: Union, Members: members}
}

// Contains returns true if a union type includes the given type as a member.
// For non-union types, always returns false.
func (t Type) Contains(other Type) bool {
	if t.Kind != Union {
		return false
	}
	for _, m := range t.Members {
		if m.Equals(other) {
			return true
		}
	}
	return false
}

// Equals returns true if two types are structurally equivalent.
func (t Type) Equals(other Type) bool {
	if t.Kind != other.Kind {
		return false
	}
	switch t.Kind {
	case BlockRef:
		if t.BlockKind != other.BlockKind {
			return false
		}
		if t.BlockKind == token.BlockSchema {
			return t.SchemaName == other.SchemaName
		}
		return true
	case List:
		if t.ElementType == nil || other.ElementType == nil {
			return t.ElementType == other.ElementType
		}
		return t.ElementType.Equals(*other.ElementType)
	case Map:
		if t.KeyType == nil || other.KeyType == nil || t.ValueType == nil || other.ValueType == nil {
			return false
		}
		return t.KeyType.Equals(*other.KeyType) && t.ValueType.Equals(*other.ValueType)
	case Union:
		if len(t.Members) != len(other.Members) {
			return false
		}
		for i := range t.Members {
			if !t.Members[i].Equals(other.Members[i]) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

// String returns a human-readable representation of a type using Orca syntax.
func (t Type) String() string {
	switch t.Kind {
	case BlockRef:
		if t.BlockKind == token.BlockSchema && t.SchemaName != "" {
			return t.SchemaName
		}
		return t.BlockKind.String()
	case List:
		if t.ElementType != nil {
			return "list[" + t.ElementType.String() + "]"
		}
		return "list"
	case Map:
		if t.ValueType != nil {
			return "map[" + t.ValueType.String() + "]"
		}
		return "map"
	case Union:
		s := ""
		for i, m := range t.Members {
			if i > 0 {
				s += " | "
			}
			s += m.String()
		}
		return s
	default:
		return "unknown"
	}
}

// IsCompatible returns true if got is type-compatible with expected.
// Handles: any (always compatible), unions (compatible if any member matches),
// numeric widening (int → float), and structural kind matching for lists/maps.
func IsCompatible(got, expected Type) bool {
	// Any is compatible with everything in both directions.
	if got.IsAny() || expected.IsAny() {
		return true
	}

	// If expected is a union, got must be compatible with at least one member.
	if expected.Kind == Union {
		for _, m := range expected.Members {
			if IsCompatible(got, m) {
				return true
			}
		}
		return false
	}

	// If got is a union, at least one member must be compatible with expected.
	if got.Kind == Union {
		for _, m := range got.Members {
			if IsCompatible(m, expected) {
				return true
			}
		}
		return false
	}

	// Numeric widening: int is compatible with float.
	if got.Equals(Int()) && expected.Equals(Float()) {
		return true
	}

	// Lists are compatible if element types are compatible (or untyped).
	if got.Kind == List && expected.Kind == List {
		if got.ElementType != nil && expected.ElementType != nil {
			return IsCompatible(*got.ElementType, *expected.ElementType)
		}
		return true
	}

	// Maps are compatible if value types are compatible (or untyped).
	if got.Kind == Map && expected.Kind == Map {
		if got.ValueType != nil && expected.ValueType != nil {
			return IsCompatible(*got.ValueType, *expected.ValueType)
		}
		return true
	}

	// BlockRef types must match by kind and schema name.
	if got.Kind == BlockRef && expected.Kind == BlockRef {
		if got.BlockKind != expected.BlockKind {
			return false
		}
		if got.BlockKind == token.BlockSchema {
			// Empty SchemaName means "any schema" — it matches any specific
			// schema. This is how `type = schema` in builtins.oc works:
			// the field accepts any schema instance (named or inline).
			if got.SchemaName == "" || expected.SchemaName == "" {
				return true
			}
			return got.SchemaName == expected.SchemaName
		}
		return true
	}

	return got.Kind == expected.Kind
}

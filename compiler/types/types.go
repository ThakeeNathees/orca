// Package types defines the Orca type system used for semantic analysis.
// All named types (str, int, model, agent, user-defined schemas) are
// represented as BlockRef with a BlockType name. Structural types
// (List, Map, Union) are separate kinds.
package types

// TypeKind classifies the broad category of a type.
type TypeKind int

const (
	// BlockRef represents a named type — primitives (str, int, etc.),
	// block types (model, agent), and user-defined schemas.
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

// String returns the human-readable name of this type kind.
func (k TypeKind) String() string {
	if s, ok := kindStrings[k]; ok {
		return s
	}
	return "unknown"
}

// BlockKind identifies which kind of block a BlockRef type points to.
type BlockKind string

const (
	BlockModel      BlockKind = "model"
	BlockAgent      BlockKind = "agent"
	BlockTool       BlockKind = "tool"
	BlockTask       BlockKind = "task"
	BlockKnowledge  BlockKind = "knowledge"
	BlockWorkflow   BlockKind = "workflow"
	BlockTrigger    BlockKind = "trigger"
	BlockInput      BlockKind = "input"
	BlockSchemaKind BlockKind = "schema"
	BlockLet        BlockKind = "let"
)

// BlockKindFromName maps a block type name string to its BlockKind constant.
var blockKindMap = map[string]BlockKind{
	"model":     BlockModel,
	"agent":     BlockAgent,
	"tool":      BlockTool,
	"task":      BlockTask,
	"knowledge": BlockKnowledge,
	"workflow":  BlockWorkflow,
	"trigger":   BlockTrigger,
	"input":     BlockInput,
	"schema":    BlockSchemaKind,
	"let":       BlockLet,
}

// Type represents a concrete type in the Orca type system.
// Named types (str, int, model, agent, user schemas) use Kind=BlockRef
// with BlockType set to the type name. Structural types use Kind=List,
// Map, or Union with their respective fields.
type Type struct {
	Kind        TypeKind
	ElementType *Type     // non-nil for List types
	KeyType     *Type     // non-nil for Map types
	ValueType   *Type     // non-nil for Map types
	BlockType   BlockKind // the type name for BlockRef types (e.g. "str", "model", "vpc_data_t")
	Members     []Type    // non-nil for Union types — the set of acceptable types
}

// typeCache stores BlockRef types keyed by name, extended on demand
// by TypeOf. Populated from blockSchemas keys at init time.
var typeCache = make(map[BlockKind]Type)

// TypeOf returns the cached BlockRef type for a given name.
// Creates and caches the type on first access if not already present.
func TypeOf(name BlockKind) Type {
	if t, ok := typeCache[name]; ok {
		return t
	}
	t := Type{Kind: BlockRef, BlockType: name}
	typeCache[name] = t
	return t
}

// IsAny returns true if this type is the "any" type (matches everything).
func (t Type) IsAny() bool {
	return t.Kind == BlockRef && t.BlockType == "any"
}

// IsNull returns true if this type is the "null" type.
func (t Type) IsNull() bool {
	return t.Kind == BlockRef && t.BlockType == "null"
}

// NewListType creates a list type with the given element type.
func NewListType(element Type) Type {
	return Type{Kind: List, ElementType: &element}
}

// NewMapType creates a map type with the given key and value types.
func NewMapType(key, value Type) Type {
	return Type{Kind: Map, KeyType: &key, ValueType: &value}
}

// NewBlockRefType creates a block reference type with the given name.
func NewBlockRefType(blockType BlockKind) Type {
	return Type{Kind: BlockRef, BlockType: blockType}
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
		return t.BlockType == other.BlockType
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
		return string(t.BlockType)
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
	if got.Kind == BlockRef && expected.Kind == BlockRef {
		if got.BlockType == "int" && expected.BlockType == "float" {
			return true
		}
	}

	// Lists are compatible by kind (element type not checked yet).
	if got.Kind == List && expected.Kind == List {
		return true
	}

	// Maps are compatible by kind (key/value types not checked yet).
	if got.Kind == Map && expected.Kind == Map {
		return true
	}

	// BlockRef types must match by name.
	if got.Kind == BlockRef && expected.Kind == BlockRef {
		return got.BlockType == expected.BlockType
	}

	return got.Kind == expected.Kind
}

// BlockKindFromName returns the BlockKind for a given block type name.
func BlockKindFromName(name string) (BlockKind, bool) {
	kind, ok := blockKindMap[name]
	return kind, ok
}

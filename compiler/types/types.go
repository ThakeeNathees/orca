// Package types defines the Orca type system used for semantic analysis.
// It provides internal type representations for block field validation
// and mappings from language-level type annotations (str, int, etc.)
// to their internal representations.
package types

// TypeKind classifies the broad category of a type.
type TypeKind int

const (
	// String represents text values like "openai".
	String TypeKind = iota
	// Int represents whole number values like 4096.
	Int
	// Float represents decimal number values like 0.7.
	Float
	// Bool represents true/false values.
	Bool
	// List represents an ordered collection of elements.
	List
	// Map represents a collection of key-value pairs.
	Map
	// Any represents an unconstrained type, used as a fallback.
	Any
	// BlockRef represents a reference to another block by name.
	BlockRef
	// Null represents the null/nil type, used in unions to mark optional fields.
	Null
	// Union represents a type that can be one of several member types.
	Union
)

// kindStrings maps each TypeKind to its human-readable name.
var kindStrings = map[TypeKind]string{
	String:   "string",
	Int:      "int",
	Float:    "float",
	Bool:     "bool",
	List:     "list",
	Map:      "map",
	Any:      "any",
	BlockRef: "block_ref",
	Null:     "null",
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
}

// Type represents a concrete type in the Orca type system.
// Primitive types (string, int, float, bool, any) use only Kind.
// Compound types use additional fields: ElementType for lists,
// KeyType/ValueType for maps, and BlockType for block references.
type Type struct {
	Kind        TypeKind
	ElementType *Type     // non-nil for List types
	KeyType     *Type     // non-nil for Map types
	ValueType   *Type     // non-nil for Map types
	BlockType   BlockKind // non-empty for BlockRef types (e.g., BlockModel, BlockAgent)
	Members     []Type    // non-nil for Union types — the set of acceptable types
}

// Pre-defined primitive type singletons for convenience.
var (
	StringType = Type{Kind: String}
	IntType    = Type{Kind: Int}
	FloatType  = Type{Kind: Float}
	BoolType   = Type{Kind: Bool}
	AnyType    = Type{Kind: Any}
	NullType   = Type{Kind: Null}
	ListType   = Type{Kind: List}
	MapType    = Type{Kind: Map}
)

// NewListType creates a list type with the given element type.
func NewListType(element Type) Type {
	return Type{Kind: List, ElementType: &element}
}

// NewMapType creates a map type with the given key and value types.
func NewMapType(key, value Type) Type {
	return Type{Kind: Map, KeyType: &key, ValueType: &value}
}

// NewBlockRefType creates a block reference type that expects a reference
// to a block of the given type (e.g., "model", "agent", "tool").
func NewBlockRefType(blockType BlockKind) Type {
	return Type{Kind: BlockRef, BlockType: blockType}
}

// NewUnionType creates a union type that accepts any of the given member types.
// For example, NewUnionType(StringType, NewBlockRefType("model")) means
// the value can be either a string literal or a reference to a model block.
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
	case BlockRef:
		return t.BlockType == other.BlockType
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

// String returns a human-readable representation of a type using Orca
// syntax (e.g. "str", "list[tool]", "str | model").
func (t Type) String() string {
	switch t.Kind {
	case String:
		return "str"
	case Int:
		return "int"
	case Float:
		return "float"
	case Bool:
		return "bool"
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
	case Any:
		return "any"
	case Null:
		return "null"
	case BlockRef:
		return string(t.BlockType)
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

// BlockKindFromName returns the BlockKind for a given block type name.
func BlockKindFromName(name string) (BlockKind, bool) {
	kind, ok := blockKindMap[name]
	return kind, ok
}

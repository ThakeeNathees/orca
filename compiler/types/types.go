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
}

// String returns the human-readable name of this type kind.
func (k TypeKind) String() string {
	if s, ok := kindStrings[k]; ok {
		return s
	}
	return "unknown"
}

// Type represents a concrete type in the Orca type system.
// Primitive types (string, int, float, bool, any) use only Kind.
// Compound types use additional fields: ElementType for lists,
// KeyType/ValueType for maps, and BlockType for block references.
type Type struct {
	Kind        TypeKind
	ElementType *Type  // non-nil for List types
	KeyType     *Type  // non-nil for Map types
	ValueType   *Type  // non-nil for Map types
	BlockType   string // non-empty for BlockRef types (e.g., "model", "agent")
}

// Pre-defined primitive type singletons for convenience.
var (
	StringType = Type{Kind: String}
	IntType    = Type{Kind: Int}
	FloatType  = Type{Kind: Float}
	BoolType   = Type{Kind: Bool}
	AnyType    = Type{Kind: Any}
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
func NewBlockRefType(blockType string) Type {
	return Type{Kind: BlockRef, BlockType: blockType}
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
	default:
		return true
	}
}

// annotationMap maps language-level type annotation strings to their
// corresponding internal types. These are the type names users write
// in .oc files (e.g., `type = str`).
var annotationMap = map[string]Type{
	"str":   StringType,
	"int":   IntType,
	"float": FloatType,
	"bool":  BoolType,
	"list":  {Kind: List},
	"map":   {Kind: Map},
	"any":   AnyType,
}

// TypeFromAnnotation converts a language-level type annotation string
// (e.g., "str", "int") to its internal Type representation. Returns
// the type and true if the annotation is valid, or zero-value and false
// if unrecognized.
func TypeFromAnnotation(annotation string) (Type, bool) {
	typ, ok := annotationMap[annotation]
	return typ, ok
}

// Package types defines the Orca type system used for semantic analysis.
// Block types (model, agent, etc.) are represented as BlockRef with a BlockKind enum.
// Primitive types (string, number, bool, any, null) are schemas defined in
// bootstrap.oc and represented as SchemaTypeOf("string"), etc. Structural types
// (List, Map, Union) are separate kinds.
package types

// TypeKind classifies the broad category of a type.
type TypeKind int

const (
	// BlockRef represents a named type — block types (model, agent)
	// and schema types (built-in primitives and user-defined schemas).
	BlockRef TypeKind = iota
	// Union represents a type that can be one of several member types.
	Union
	// TODO: List and Map should be a BlockRef type, but for now
	// We keep them as separate types, because of their generic nature.
	// List represents an ordered collection of elements.
	List
	// Map represents a collection of key-value pairs.
	Map
	// Callable represents a function type with parameter types and a return type.
	Callable
)

// kindStrings maps each TypeKind to its human-readable name.
var kindStrings = map[TypeKind]string{
	BlockRef: "blockref",
	List:     "list",
	Map:      "map",
	Union:    "union",
	Callable: "callable",
}

// String returns the human-readable name of this type kind.
func (k TypeKind) String() string {
	if s, ok := kindStrings[k]; ok {
		return s
	}
	return "unknown"
}

// Type represents a concrete type in the Orca type system.
// Block types (model, agent) use Kind=BlockRef with BlockKind set.
// Schema types (string, int, user schemas) use Kind=BlockRef with
// BlockKind=BlockSchema and SchemaName set.
// Structural types use Kind=List, Map, or Union.
type Type struct {
	Kind TypeKind

	// +------------+---------------------+-------------------------------------------------+
	// | Expression | Block               | Meaning                                         |
	// +------------+---------------------+-------------------------------------------------+
	// | string     | schema string {}    | string (identifier) points to block schema string {} |
	// | "foo"      | string "foo" {}     | "foo" (literal) consecptually points to string "foo" {} block
	// |            |                     |                                                 |
	// | bool       | schema bool {}      | bool (identifier) points to block schema bool {}
	// | true       | bool true {}        | true (identifier) points to bool true {} block  |
	// |            |                     |                                                 |
	// | agent      | schema agent {}     | agent (identifier) points to block schema agent {} block
	// | reseacher  | agent researcher {} | researcher (identifier) points to block agent researcher {} block
	// |            |                     |                                                 |
	// | schema     | schema schema {}    | schema (identifier) points to block schema schema {} block
	// +------------+---------------------+-------------------------------------------------+

	// non-nil for block ref types. This will be resolved to Block in a later phase so this is
	// a lazy binding.
	BlockName string
	Block     *BlockSchema // the defined block schema

	// non-nil for Union types — the set of acceptable types
	Members []Type

	// non-nil for List types
	ElementType *Type

	// non-nil for Map types
	KeyType   *Type
	ValueType *Type

	// non-nil for Callable types
	ParamTypes []Type
	ReturnType *Type
}

// NewListType creates a list type with the given element type.
func NewListType(element Type) Type {
	return Type{Kind: List, ElementType: &element}
}

// NewMapType creates a map type with the given key and value types.
func NewMapType(key, value Type) Type {
	return Type{Kind: Map, KeyType: &key, ValueType: &value}
}

// NewCallableType creates a callable type with the given parameter and return types.
func NewCallableType(params []Type, ret Type) Type {
	return Type{Kind: Callable, ParamTypes: params, ReturnType: &ret}
}

// NewUnionType creates a union type that accepts any of the given member types.
func NewUnionType(members ...Type) Type {
	return Type{Kind: Union, Members: members}
}

// NewBlockRefType creates a block reference type with the given BlockKind.
func NewBlockRefType(blockName string, block *BlockSchema) Type {
	return Type{Kind: BlockRef, BlockName: blockName, Block: block}
}

// FIXME: I dont like null/any being some special thing and hardcoded, fix it.
// IsAny returns true if this type is the "any" type (matches everything).
func (t Type) IsAny() bool {
	if t.Kind != BlockRef {
		return false
	}
	if t.Block != nil {
		return t.Block.BlockName == "any"
	}
	// Unresolved identifier "any" (bootstrap / before schema resolution).
	return t.BlockName == "any"
}

// IsNull returns true if this type is the "null" type.
func (t Type) IsNull() bool {
	if t.Kind != BlockRef {
		return false
	}
	if t.Block != nil {
		return t.Block.BlockName == "null"
	}
	// Unresolved identifier "null" so NewFieldSchema can strip `| null` from unions.
	return t.BlockName == "null"
}

// String returns a human-readable representation of a type using Orca syntax.
func (t Type) String() string {
	switch t.Kind {
	case BlockRef:
		if t.Block != nil {
			return t.Block.BlockName
		}
		// Unresolved ref: use the identifier / block name (same as Equals for lazy refs).
		if t.BlockName != "" {
			return t.BlockName
		}
		return "<type:blockref>"
	case List:
		if t.ElementType != nil {
			return "list[" + t.ElementType.String() + "]"
		}
		return "list"
	case Map:
		if t.KeyType != nil && t.ValueType != nil {
			return "map[" + t.KeyType.String() + ", " + t.ValueType.String() + "]"
		}
		return "map"
	case Callable:
		s := "callable["
		for i, p := range t.ParamTypes {
			if i > 0 {
				s += ", "
			}
			s += p.String()
		}
		if len(t.ParamTypes) > 0 {
			s += ", "
		}
		if t.ReturnType != nil {
			s += t.ReturnType.String()
		}
		s += "]"
		return s
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
		return "<type:unknown>"
	}
}

// IsCompatible returns true if got is type-compatible with expected.
// Handles: any (always compatible), unions (compatible if any member matches),
// numeric widening (int → float), and structural kind matching for lists/maps.
func IsCompatible(got Type, expected Type) bool {

	// Any is compatible with everything in both directions.
	if got.IsAny() || expected.IsAny() {
		return true
	}

	// Unresolved block refs with the same name are the same type (literals, IdentType in
	// bootstrap mode). Without this, IsCompatible never succeeds for lazy string/string or
	// number/number, and arithmeticResultType falls through to any.
	if got.Kind == BlockRef && expected.Kind == BlockRef {
		if got.Block == nil && expected.Block == nil && got.BlockName == expected.BlockName {
			return true
		}
	}

	// We're expecting type `exp` (kind should be schema), and got value `vgot` of kind `kgot`.
	// And they are defined as:
	//
	//   schema exp {...}
	//   schema kgot {...}
	//   kgot vgot {...}
	//
	// And consider a scnario:
	//   schema some_s {
	//     some_field = exp
	//   }
	//   some_s some_v {
	//     some_field = vgot  // <-- We need to validate got is compatible with exp
	//   }
	//
	// ExprType(vgot) -> Type(BlockRef(schema kgot {...}))
	// NOTE: kgot is the `got` of this function.
	//
	// Now for them to be compatible: kgot == exp
	// i.e. block `schema kgot {...}`'s Schema should be `schema exp {...}`
	//
	if expected.Kind == BlockRef && expected.Block != nil {

		// First things first, if exp kind is not schema we just check they are the same.
		// example:
		//   schema model {
		//       provider = "openai" | "anthropic" | "google"
		//   }
		//
		//   moddel mymodel {
		//       provider = "openapi"  <-- Not compatible with "openai"
		//   }
		if expected.Block.Ast.Kind != BlockKindSchema {
			// TODO: Figureout this (maybe attach the literal value
			// to type or literals are its own type like python).
			return false
		}

		// A bare primitive schema (e.g. `schema list {}`, `schema map {}`,
		// `schema callable {}`) accepts any value whose native kind matches.
		// This handles cases like `invoke = callable` accepting a lambda, or
		// a hypothetical `items = list` accepting a list literal.
		if got.Kind != BlockRef {
			return kindStrings[got.Kind] == expected.BlockName
		}

		// When got has an unresolved block pointer (e.g. from an inline block
		// expression), fall back to name-based matching against the expected schema.
		if got.Block == nil {
			return got.BlockName == expected.BlockName
		}

		// Validate: block `schema kgot {...}`'s Schema should be `schema exp {...}`
		if got.Block == expected.Block {
			return true
		}

		// When got is an instance block (e.g. `model o {}`), check if its
		// schema matches expected (e.g. `schema model {}`).
		if got.Block.Schema != nil && got.Block.Schema == expected.Block {
			return true
		}

		return false
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

	// TODO: If got is a union, and one of the member matching with expected
	// That should be ok with a warning.
	// If got is a union, at least one member must be compatible with expected.
	if got.Kind == Union {
		for _, m := range got.Members {
			if IsCompatible(m, expected) {
				return true
			}
		}
		return false
	}

	// Lists are compatible if element types are compatible (or untyped).
	if got.Kind == List && expected.Kind == List {
		if got.ElementType != nil && expected.ElementType != nil {
			// FIXME:
			//
			// list<Cat> is actually not compatible with list<Pet> even thought
			// Cat is a subtype of Pet. (see: https://en.wikipedia.org/wiki/Type_variance)
			//
			// Here both of the bellow insertions are semantically valid:
			//   pets.insert(cat)
			//   pets.insert(dog)
			//
			// However if we say list<Cat> is a list<Pet> then the above will be.
			//   cats.insert(cat)
			//   cats.insert(dog)  <---- OOPS
			//
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

	// Callables: a bare callable (no params/return) accepts any callable.
	// Typed callables check param count, param types, and return type.
	if got.Kind == Callable && expected.Kind == Callable {
		// Bare callable accepts any callable.
		if len(expected.ParamTypes) == 0 && expected.ReturnType == nil {
			return true
		}
		if len(got.ParamTypes) != len(expected.ParamTypes) {
			return false
		}
		for i := range got.ParamTypes {
			if !IsCompatible(got.ParamTypes[i], expected.ParamTypes[i]) {
				return false
			}
		}
		if got.ReturnType != nil && expected.ReturnType != nil {
			return IsCompatible(*got.ReturnType, *expected.ReturnType)
		}
		return true
	}

	// Unreachable code.
	return false
}

// Equals returns true if two types are structurally equivalent.
func (t Type) Equals(other Type) bool {
	if t.Kind != other.Kind {
		return false
	}
	switch t.Kind {
	case BlockRef:
		// Same resolved schema block (pointer identity), or matching lazy refs with no Block yet.
		switch {
		case t.Block != nil && other.Block != nil:
			return t.Block == other.Block
		case t.Block == nil && other.Block == nil:
			return t.BlockName == other.BlockName
		default:
			return false
		}
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
	case Callable:
		if len(t.ParamTypes) != len(other.ParamTypes) {
			return false
		}
		for i := range t.ParamTypes {
			if !t.ParamTypes[i].Equals(other.ParamTypes[i]) {
				return false
			}
		}
		if t.ReturnType == nil || other.ReturnType == nil {
			return t.ReturnType == other.ReturnType
		}
		return t.ReturnType.Equals(*other.ReturnType)
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

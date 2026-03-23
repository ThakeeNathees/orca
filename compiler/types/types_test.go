package types

import "testing"

// TestTypeKindString verifies that each TypeKind has a correct string representation.
func TestTypeKindString(t *testing.T) {
	tests := []struct {
		name     string
		kind     TypeKind
		expected string
	}{
		{"block ref kind", BlockRef, "block_ref"},
		{"list kind", List, "list"},
		{"map kind", Map, "map"},
		{"union kind", Union, "union"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.expected {
				t.Errorf("TypeKind.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestTypeOf verifies that TypeOf returns cached BlockRef types.
func TestTypeOf(t *testing.T) {
	tests := []struct {
		name      string
		input     BlockKind
		blockType BlockKind
	}{
		{"str", "str", "str"},
		{"int", "int", "int"},
		{"float", "float", "float"},
		{"bool", "bool", "bool"},
		{"any", "any", "any"},
		{"null", "null", "null"},
		{"model", BlockModel, BlockModel},
		{"user schema", "vpc_data_t", "vpc_data_t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := TypeOf(tt.input)
			if typ.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", typ.Kind)
			}
			if typ.BlockType != tt.blockType {
				t.Errorf("BlockType = %q, want %q", typ.BlockType, tt.blockType)
			}
		})
	}
}

// TestTypeOfCaching verifies that TypeOf returns equal types.
func TestTypeOfCaching(t *testing.T) {
	a := TypeOf("str")
	b := TypeOf("str")
	if !a.Equals(b) {
		t.Error("TypeOf should return equal values")
	}
}

// TestListType verifies list type construction with element types.
func TestListType(t *testing.T) {
	tests := []struct {
		name       string
		elementBT  BlockKind
	}{
		{"list of strings", "str"},
		{"list of ints", "int"},
		{"list of any", "any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listType := NewListType(TypeOf(tt.elementBT))
			if listType.Kind != List {
				t.Errorf("Kind = %v, want List", listType.Kind)
			}
			if listType.ElementType == nil {
				t.Fatal("ElementType should not be nil")
			}
			if listType.ElementType.BlockType != tt.elementBT {
				t.Errorf("ElementType.BlockType = %q, want %q", listType.ElementType.BlockType, tt.elementBT)
			}
		})
	}
}

// TestMapType verifies map type construction with key and value types.
func TestMapType(t *testing.T) {
	tests := []struct {
		name    string
		keyBT   BlockKind
		valueBT BlockKind
	}{
		{"map str to str", "str", "str"},
		{"map str to int", "str", "int"},
		{"map str to any", "str", "any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapType := NewMapType(TypeOf(tt.keyBT), TypeOf(tt.valueBT))
			if mapType.Kind != Map {
				t.Errorf("Kind = %v, want Map", mapType.Kind)
			}
			if mapType.KeyType == nil || mapType.ValueType == nil {
				t.Fatal("KeyType and ValueType should not be nil")
			}
			if mapType.KeyType.BlockType != tt.keyBT {
				t.Errorf("KeyType.BlockType = %q, want %q", mapType.KeyType.BlockType, tt.keyBT)
			}
			if mapType.ValueType.BlockType != tt.valueBT {
				t.Errorf("ValueType.BlockType = %q, want %q", mapType.ValueType.BlockType, tt.valueBT)
			}
		})
	}
}

// TestBlockRefType verifies block reference type construction.
func TestBlockRefType(t *testing.T) {
	tests := []struct {
		name      string
		blockType BlockKind
	}{
		{"model ref", BlockModel},
		{"agent ref", BlockAgent},
		{"tool ref", BlockTool},
		{"str ref", "str"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refType := NewBlockRefType(tt.blockType)
			if refType.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", refType.Kind)
			}
			if refType.BlockType != tt.blockType {
				t.Errorf("BlockType = %q, want %q", refType.BlockType, tt.blockType)
			}
		})
	}
}

// TestTypeEquals verifies type equality comparison.
func TestTypeEquals(t *testing.T) {
	str := TypeOf("str")
	intT := TypeOf("int")
	floatT := TypeOf("float")

	tests := []struct {
		name     string
		a        Type
		b        Type
		expected bool
	}{
		{"same named types", str, str, true},
		{"different named types", str, intT, false},
		{"list vs list same element", NewListType(str), NewListType(str), true},
		{"list vs list diff element", NewListType(str), NewListType(intT), false},
		{"map vs map same", NewMapType(str, intT), NewMapType(str, intT), true},
		{"map vs map diff value", NewMapType(str, intT), NewMapType(str, str), false},
		{"block ref same", NewBlockRefType(BlockModel), NewBlockRefType(BlockModel), true},
		{"block ref diff", NewBlockRefType(BlockModel), NewBlockRefType(BlockAgent), false},
		{"any equals any", TypeOf("any"), TypeOf("any"), true},
		{"named vs list", str, NewListType(str), false},
		{"union same members", NewUnionType(str, NewBlockRefType(BlockModel)), NewUnionType(str, NewBlockRefType(BlockModel)), true},
		{"union diff members", NewUnionType(str, intT), NewUnionType(str, floatT), false},
		{"union diff length", NewUnionType(str, intT), NewUnionType(str), false},
		{"union vs named", NewUnionType(str), str, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equals(tt.b); got != tt.expected {
				t.Errorf("Equals() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestUnionType verifies union type construction and member access.
func TestUnionType(t *testing.T) {
	str := TypeOf("str")
	intT := TypeOf("int")
	floatT := TypeOf("float")

	tests := []struct {
		name      string
		members   []Type
		expectLen int
		expectBTs []BlockKind
	}{
		{"str or model", []Type{str, NewBlockRefType(BlockModel)}, 2, []BlockKind{"str", BlockModel}},
		{"str or int or float", []Type{str, intT, floatT}, 3, []BlockKind{"str", "int", "float"}},
		{"single member", []Type{str}, 1, []BlockKind{"str"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			union := NewUnionType(tt.members...)
			if union.Kind != Union {
				t.Errorf("Kind = %v, want Union", union.Kind)
			}
			if len(union.Members) != tt.expectLen {
				t.Fatalf("len(Members) = %d, want %d", len(union.Members), tt.expectLen)
			}
			for i, bt := range tt.expectBTs {
				if union.Members[i].BlockType != bt {
					t.Errorf("Members[%d].BlockType = %q, want %q", i, union.Members[i].BlockType, bt)
				}
			}
		})
	}
}

// TestUnionTypeContains verifies that Contains checks membership correctly.
func TestUnionTypeContains(t *testing.T) {
	str := TypeOf("str")
	union := NewUnionType(str, NewBlockRefType(BlockModel))

	tests := []struct {
		name     string
		check    Type
		expected bool
	}{
		{"contains str", str, true},
		{"contains model", NewBlockRefType(BlockModel), true},
		{"does not contain int", TypeOf("int"), false},
		{"does not contain agent", NewBlockRefType(BlockAgent), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := union.Contains(tt.check); got != tt.expected {
				t.Errorf("Contains() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestIsAny verifies the IsAny helper.
func TestIsAny(t *testing.T) {
	if !TypeOf("any").IsAny() {
		t.Error("TypeOf(any).IsAny() should be true")
	}
	if TypeOf("str").IsAny() {
		t.Error("TypeOf(str).IsAny() should be false")
	}
}

// TestIsNull verifies the IsNull helper.
func TestIsNull(t *testing.T) {
	if !TypeOf("null").IsNull() {
		t.Error("TypeOf(null).IsNull() should be true")
	}
	if TypeOf("str").IsNull() {
		t.Error("TypeOf(str).IsNull() should be false")
	}
}

// TestTypeOfUserDefinedSchema verifies that TypeOf works for user-defined
// schema names the same way it works for primitives.
func TestTypeOfUserDefinedSchema(t *testing.T) {
	vpc := TypeOf("vpc_data_t")
	str := TypeOf("str")

	// Both are BlockRef — no distinction.
	if vpc.Kind != str.Kind {
		t.Errorf("vpc Kind = %v, str Kind = %v — should be equal", vpc.Kind, str.Kind)
	}
	// But different names.
	if vpc.Equals(str) {
		t.Error("vpc_data_t should not equal str")
	}
	// Same name returns equal.
	if !vpc.Equals(TypeOf("vpc_data_t")) {
		t.Error("vpc_data_t should equal vpc_data_t")
	}
}

// TestTypeStringRendering verifies that Type.String() produces correct
// Orca syntax for all type kinds.
func TestTypeStringRendering(t *testing.T) {
	tests := []struct {
		name     string
		typ      Type
		expected string
	}{
		{"str", TypeOf("str"), "str"},
		{"int", TypeOf("int"), "int"},
		{"null", TypeOf("null"), "null"},
		{"any", TypeOf("any"), "any"},
		{"model", NewBlockRefType(BlockModel), "model"},
		{"user schema", TypeOf("vpc_data_t"), "vpc_data_t"},
		{"list", Type{Kind: List}, "list"},
		{"list[str]", NewListType(TypeOf("str")), "list[str]"},
		{"map", Type{Kind: Map}, "map"},
		{"map[int]", NewMapType(TypeOf("str"), TypeOf("int")), "map[int]"},
		{"union str | int", NewUnionType(TypeOf("str"), TypeOf("int")), "str | int"},
		{"union str | model | null", NewUnionType(TypeOf("str"), NewBlockRefType(BlockModel), TypeOf("null")), "str | model | null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.typ.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestPrimitivesInUnion verifies that primitives work in unions the same
// way block refs do — no special casing.
func TestPrimitivesInUnion(t *testing.T) {
	union := NewUnionType(TypeOf("str"), TypeOf("int"), TypeOf("null"))

	tests := []struct {
		name     string
		check    Type
		expected bool
	}{
		{"contains str", TypeOf("str"), true},
		{"contains int", TypeOf("int"), true},
		{"contains null", TypeOf("null"), true},
		{"does not contain float", TypeOf("float"), false},
		{"does not contain model", NewBlockRefType(BlockModel), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := union.Contains(tt.check); got != tt.expected {
				t.Errorf("Contains() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestPrimitiveAndBlockRefEquality verifies that primitives and block types
// use the same equality mechanism — name comparison.
func TestPrimitiveAndBlockRefEquality(t *testing.T) {
	tests := []struct {
		name     string
		a        Type
		b        Type
		expected bool
	}{
		{"str == str", TypeOf("str"), TypeOf("str"), true},
		{"str != int", TypeOf("str"), TypeOf("int"), false},
		{"str != model", TypeOf("str"), NewBlockRefType(BlockModel), false},
		{"model == model", NewBlockRefType(BlockModel), NewBlockRefType(BlockModel), true},
		{"list[str] == list[str]", NewListType(TypeOf("str")), NewListType(TypeOf("str")), true},
		{"list[str] != list[int]", NewListType(TypeOf("str")), NewListType(TypeOf("int")), false},
		{"list[model] == list[model]", NewListType(NewBlockRefType(BlockModel)), NewListType(NewBlockRefType(BlockModel)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equals(tt.b); got != tt.expected {
				t.Errorf("Equals() = %v, want %v", got, tt.expected)
			}
		})
	}
}

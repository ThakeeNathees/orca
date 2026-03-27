package types

import (
	"testing"

	"github.com/thakee/orca/compiler/token"
)

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
		input     token.BlockKind
		blockKind token.BlockKind
	}{
		{"str", token.BlockStr, token.BlockStr},
		{"int", token.BlockInt, token.BlockInt},
		{"float", token.BlockFloat, token.BlockFloat},
		{"bool", token.BlockBool, token.BlockBool},
		{"any", token.BlockAny, token.BlockAny},
		{"null", token.BlockNull, token.BlockNull},
		{"model", token.BlockModel, token.BlockModel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := TypeOf(tt.input)
			if typ.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", typ.Kind)
			}
			if typ.BlockKind != tt.blockKind {
				t.Errorf("BlockKind = %v, want %v", typ.BlockKind, tt.blockKind)
			}
		})
	}
}

// TestTypeOfCaching verifies that TypeOf returns equal types.
func TestTypeOfCaching(t *testing.T) {
	a := TypeOf(token.BlockStr)
	b := TypeOf(token.BlockStr)
	if !a.Equals(b) {
		t.Error("TypeOf should return equal values")
	}
}

// TestListType verifies list type construction with element types.
func TestListType(t *testing.T) {
	tests := []struct {
		name      string
		elementBK token.BlockKind
	}{
		{"list of strings", token.BlockStr},
		{"list of ints", token.BlockInt},
		{"list of any", token.BlockAny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listType := NewListType(TypeOf(tt.elementBK))
			if listType.Kind != List {
				t.Errorf("Kind = %v, want List", listType.Kind)
			}
			if listType.ElementType == nil {
				t.Fatal("ElementType should not be nil")
			}
			if listType.ElementType.BlockKind != tt.elementBK {
				t.Errorf("ElementType.BlockKind = %v, want %v", listType.ElementType.BlockKind, tt.elementBK)
			}
		})
	}
}

// TestMapType verifies map type construction with key and value types.
func TestMapType(t *testing.T) {
	tests := []struct {
		name    string
		keyBK   token.BlockKind
		valueBK token.BlockKind
	}{
		{"map str to str", token.BlockStr, token.BlockStr},
		{"map str to int", token.BlockStr, token.BlockInt},
		{"map str to any", token.BlockStr, token.BlockAny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapType := NewMapType(TypeOf(tt.keyBK), TypeOf(tt.valueBK))
			if mapType.Kind != Map {
				t.Errorf("Kind = %v, want Map", mapType.Kind)
			}
			if mapType.KeyType == nil || mapType.ValueType == nil {
				t.Fatal("KeyType and ValueType should not be nil")
			}
			if mapType.KeyType.BlockKind != tt.keyBK {
				t.Errorf("KeyType.BlockKind = %v, want %v", mapType.KeyType.BlockKind, tt.keyBK)
			}
			if mapType.ValueType.BlockKind != tt.valueBK {
				t.Errorf("ValueType.BlockKind = %v, want %v", mapType.ValueType.BlockKind, tt.valueBK)
			}
		})
	}
}

// TestBlockRefType verifies block reference type construction.
func TestBlockRefType(t *testing.T) {
	tests := []struct {
		name      string
		blockKind token.BlockKind
	}{
		{"model ref", token.BlockModel},
		{"agent ref", token.BlockAgent},
		{"tool ref", token.BlockTool},
		{"str ref", token.BlockStr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refType := NewBlockRefType(tt.blockKind)
			if refType.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", refType.Kind)
			}
			if refType.BlockKind != tt.blockKind {
				t.Errorf("BlockKind = %v, want %v", refType.BlockKind, tt.blockKind)
			}
		})
	}
}

// TestTypeEquals verifies type equality comparison.
func TestTypeEquals(t *testing.T) {
	str := TypeOf(token.BlockStr)
	intT := TypeOf(token.BlockInt)
	floatT := TypeOf(token.BlockFloat)

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
		{"block ref same", NewBlockRefType(token.BlockModel), NewBlockRefType(token.BlockModel), true},
		{"block ref diff", NewBlockRefType(token.BlockModel), NewBlockRefType(token.BlockAgent), false},
		{"any equals any", TypeOf(token.BlockAny), TypeOf(token.BlockAny), true},
		{"named vs list", str, NewListType(str), false},
		{"union same members", NewUnionType(str, NewBlockRefType(token.BlockModel)), NewUnionType(str, NewBlockRefType(token.BlockModel)), true},
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
	str := TypeOf(token.BlockStr)
	intT := TypeOf(token.BlockInt)
	floatT := TypeOf(token.BlockFloat)

	tests := []struct {
		name      string
		members   []Type
		expectLen int
		expectBKs []token.BlockKind
	}{
		{"str or model", []Type{str, NewBlockRefType(token.BlockModel)}, 2, []token.BlockKind{token.BlockStr, token.BlockModel}},
		{"str or int or float", []Type{str, intT, floatT}, 3, []token.BlockKind{token.BlockStr, token.BlockInt, token.BlockFloat}},
		{"single member", []Type{str}, 1, []token.BlockKind{token.BlockStr}},
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
			for i, bk := range tt.expectBKs {
				if union.Members[i].BlockKind != bk {
					t.Errorf("Members[%d].BlockKind = %v, want %v", i, union.Members[i].BlockKind, bk)
				}
			}
		})
	}
}

// TestUnionTypeContains verifies that Contains checks membership correctly.
func TestUnionTypeContains(t *testing.T) {
	str := TypeOf(token.BlockStr)
	union := NewUnionType(str, NewBlockRefType(token.BlockModel))

	tests := []struct {
		name     string
		check    Type
		expected bool
	}{
		{"contains str", str, true},
		{"contains model", NewBlockRefType(token.BlockModel), true},
		{"does not contain int", TypeOf(token.BlockInt), false},
		{"does not contain agent", NewBlockRefType(token.BlockAgent), false},
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
	if !TypeOf(token.BlockAny).IsAny() {
		t.Error("TypeOf(any).IsAny() should be true")
	}
	if TypeOf(token.BlockStr).IsAny() {
		t.Error("TypeOf(str).IsAny() should be false")
	}
}

// TestIsNull verifies the IsNull helper.
func TestIsNull(t *testing.T) {
	if !TypeOf(token.BlockNull).IsNull() {
		t.Error("TypeOf(null).IsNull() should be true")
	}
	if TypeOf(token.BlockStr).IsNull() {
		t.Error("TypeOf(str).IsNull() should be false")
	}
}

// TestTypeOfUserDefinedSchema verifies that SchemaTypeOf works for user-defined
// schema names.
func TestTypeOfUserDefinedSchema(t *testing.T) {
	vpc := SchemaTypeOf("vpc_data_t")
	str := TypeOf(token.BlockStr)

	// Both are BlockRef — no distinction in Kind.
	if vpc.Kind != str.Kind {
		t.Errorf("vpc Kind = %v, str Kind = %v — should be equal", vpc.Kind, str.Kind)
	}
	// But different types.
	if vpc.Equals(str) {
		t.Error("vpc_data_t should not equal str")
	}
	// Same name returns equal.
	if !vpc.Equals(SchemaTypeOf("vpc_data_t")) {
		t.Error("vpc_data_t should equal vpc_data_t")
	}
}

// TestSchemaTypeOfFields verifies SchemaTypeOf sets BlockKind and SchemaName correctly.
func TestSchemaTypeOfFields(t *testing.T) {
	typ := SchemaTypeOf("my_schema")
	if typ.Kind != BlockRef {
		t.Errorf("Kind = %v, want BlockRef", typ.Kind)
	}
	if typ.BlockKind != token.BlockSchema {
		t.Errorf("BlockKind = %v, want BlockSchema", typ.BlockKind)
	}
	if typ.SchemaName != "my_schema" {
		t.Errorf("SchemaName = %q, want %q", typ.SchemaName, "my_schema")
	}
}

// TestSchemaTypeOfCaching verifies SchemaTypeOf returns equal types for the same name.
func TestSchemaTypeOfCaching(t *testing.T) {
	a := SchemaTypeOf("cached_schema")
	b := SchemaTypeOf("cached_schema")
	if !a.Equals(b) {
		t.Error("SchemaTypeOf should return equal values for the same name")
	}
}

// TestSchemaTypeEquality verifies that different user schemas are not equal
// even though both have BlockKind == BlockSchema.
func TestSchemaTypeEquality(t *testing.T) {
	tests := []struct {
		name   string
		a      Type
		b      Type
		expect bool
	}{
		{"same schema", SchemaTypeOf("foo"), SchemaTypeOf("foo"), true},
		{"different schemas", SchemaTypeOf("foo"), SchemaTypeOf("bar"), false},
		{"schema vs model", SchemaTypeOf("foo"), NewBlockRefType(token.BlockModel), false},
		{"schema vs bare BlockSchema", SchemaTypeOf("foo"), TypeOf(token.BlockSchema), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equals(tt.b); got != tt.expect {
				t.Errorf("Equals() = %v, want %v", got, tt.expect)
			}
		})
	}
}

// TestSchemaTypeCompatibility verifies IsCompatible for user-defined schema types.
func TestSchemaTypeCompatibility(t *testing.T) {
	fooSchema := SchemaTypeOf("foo_t")
	barSchema := SchemaTypeOf("bar_t")
	anyT := TypeOf(token.BlockAny)

	tests := []struct {
		name   string
		got    Type
		expect Type
		result bool
	}{
		{"same schema compatible", fooSchema, fooSchema, true},
		{"different schemas incompatible", fooSchema, barSchema, false},
		{"schema compatible with any", fooSchema, anyT, true},
		{"any compatible with schema", anyT, fooSchema, true},
		{"schema incompatible with str", fooSchema, TypeOf(token.BlockStr), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCompatible(tt.got, tt.expect); got != tt.result {
				t.Errorf("IsCompatible(%s, %s) = %v, want %v",
					tt.got.String(), tt.expect.String(), got, tt.result)
			}
		})
	}
}

// TestSchemaTypeString verifies String() for user schema types.
func TestSchemaTypeString(t *testing.T) {
	tests := []struct {
		name   string
		typ    Type
		expect string
	}{
		{"named schema", SchemaTypeOf("vpc_data_t"), "vpc_data_t"},
		{"bare BlockSchema", TypeOf(token.BlockSchema), "schema"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.typ.String(); got != tt.expect {
				t.Errorf("String() = %q, want %q", got, tt.expect)
			}
		})
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
		{"str", TypeOf(token.BlockStr), "str"},
		{"int", TypeOf(token.BlockInt), "int"},
		{"null", TypeOf(token.BlockNull), "null"},
		{"any", TypeOf(token.BlockAny), "any"},
		{"model", NewBlockRefType(token.BlockModel), "model"},
		{"user schema", SchemaTypeOf("vpc_data_t"), "vpc_data_t"},
		{"list", Type{Kind: List}, "list"},
		{"list[str]", NewListType(TypeOf(token.BlockStr)), "list[str]"},
		{"map", Type{Kind: Map}, "map"},
		{"map[int]", NewMapType(TypeOf(token.BlockStr), TypeOf(token.BlockInt)), "map[int]"},
		{"union str | int", NewUnionType(TypeOf(token.BlockStr), TypeOf(token.BlockInt)), "str | int"},
		{"union str | model | null", NewUnionType(TypeOf(token.BlockStr), NewBlockRefType(token.BlockModel), TypeOf(token.BlockNull)), "str | model | null"},
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
	union := NewUnionType(TypeOf(token.BlockStr), TypeOf(token.BlockInt), TypeOf(token.BlockNull))

	tests := []struct {
		name     string
		check    Type
		expected bool
	}{
		{"contains str", TypeOf(token.BlockStr), true},
		{"contains int", TypeOf(token.BlockInt), true},
		{"contains null", TypeOf(token.BlockNull), true},
		{"does not contain float", TypeOf(token.BlockFloat), false},
		{"does not contain model", NewBlockRefType(token.BlockModel), false},
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
// use the same equality mechanism.
func TestPrimitiveAndBlockRefEquality(t *testing.T) {
	tests := []struct {
		name     string
		a        Type
		b        Type
		expected bool
	}{
		{"str == str", TypeOf(token.BlockStr), TypeOf(token.BlockStr), true},
		{"str != int", TypeOf(token.BlockStr), TypeOf(token.BlockInt), false},
		{"str != model", TypeOf(token.BlockStr), NewBlockRefType(token.BlockModel), false},
		{"model == model", NewBlockRefType(token.BlockModel), NewBlockRefType(token.BlockModel), true},
		{"list[str] == list[str]", NewListType(TypeOf(token.BlockStr)), NewListType(TypeOf(token.BlockStr)), true},
		{"list[str] != list[int]", NewListType(TypeOf(token.BlockStr)), NewListType(TypeOf(token.BlockInt)), false},
		{"list[model] == list[model]", NewListType(NewBlockRefType(token.BlockModel)), NewListType(NewBlockRefType(token.BlockModel)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equals(tt.b); got != tt.expected {
				t.Errorf("Equals() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Coverage: exercises TypeKind.String for unknown/invalid values.
func TestTypeKindStringUnknown(t *testing.T) {
	tests := []struct {
		name     string
		kind     TypeKind
		expected string
	}{
		{"unknown kind 99", TypeKind(99), "unknown"},
		{"unknown kind -1", TypeKind(-1), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.expected {
				t.Errorf("TypeKind(%d).String() = %q, want %q", tt.kind, got, tt.expected)
			}
		})
	}
}

// TestIsCompatible verifies type compatibility checks including unions,
// numeric widening (int -> float), and any-type matching.
func TestIsCompatible(t *testing.T) {
	str := TypeOf(token.BlockStr)
	intT := TypeOf(token.BlockInt)
	floatT := TypeOf(token.BlockFloat)
	boolT := TypeOf(token.BlockBool)
	anyT := TypeOf(token.BlockAny)
	listStr := NewListType(str)
	listInt := NewListType(intT)
	mapStrStr := NewMapType(str, str)
	unionListNull := NewUnionType(NewListType(TypeOf(token.BlockTool)), TypeOf(token.BlockNull))

	tests := []struct {
		name     string
		got      Type
		expected Type
		result   bool
	}{
		// Exact matches.
		{"str compatible with str", str, str, true},
		{"int compatible with int", intT, intT, true},
		{"list[str] compatible with list[str]", listStr, listStr, true},

		// Mismatches.
		{"str not compatible with int", str, intT, false},
		{"bool not compatible with int", boolT, intT, false},
		{"list not compatible with map", listStr, mapStrStr, false},

		// Numeric widening: int -> float.
		{"int compatible with float", intT, floatT, true},
		{"float not compatible with int", floatT, intT, false},

		// Any matches everything.
		{"any compatible with str", anyT, str, true},
		{"str compatible with any", str, anyT, true},
		{"any compatible with any", anyT, anyT, true},

		// Union: got is a union containing a compatible member.
		{"union(list,null) compatible with list", unionListNull, NewListType(TypeOf(token.BlockTool)), true},
		{"union(list,null) not compatible with str", unionListNull, str, false},

		// Union: expected is a union.
		{"str compatible with union(str,int)", str, NewUnionType(str, intT), true},
		{"bool not compatible with union(str,int)", boolT, NewUnionType(str, intT), false},

		// List kind compatibility (ignoring element type).
		{"list[str] compatible with list[int]", listStr, listInt, true},

		// Null compatible with null.
		{"null compatible with null", TypeOf(token.BlockNull), TypeOf(token.BlockNull), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCompatible(tt.got, tt.expected); got != tt.result {
				t.Errorf("IsCompatible(%s, %s) = %v, want %v",
					tt.got.String(), tt.expected.String(), got, tt.result)
			}
		})
	}
}

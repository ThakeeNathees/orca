package types

import "testing"

// TestTypeKindString verifies that each TypeKind has a correct string representation.
func TestTypeKindString(t *testing.T) {
	tests := []struct {
		name     string
		kind     TypeKind
		expected string
	}{
		{"string kind", String, "string"},
		{"int kind", Int, "int"},
		{"float kind", Float, "float"},
		{"bool kind", Bool, "bool"},
		{"list kind", List, "list"},
		{"map kind", Map, "map"},
		{"any kind", Any, "any"},
		{"block ref kind", BlockRef, "block_ref"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.expected {
				t.Errorf("TypeKind.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestNewType verifies basic type construction.
func TestNewType(t *testing.T) {
	tests := []struct {
		name         string
		typ          Type
		expectedKind TypeKind
	}{
		{"string type", StringType, String},
		{"int type", IntType, Int},
		{"float type", FloatType, Float},
		{"bool type", BoolType, Bool},
		{"any type", AnyType, Any},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.typ.Kind != tt.expectedKind {
				t.Errorf("Type.Kind = %v, want %v", tt.typ.Kind, tt.expectedKind)
			}
			if tt.typ.ElementType != nil {
				t.Errorf("Type.ElementType should be nil for primitive type")
			}
		})
	}
}

// TestListType verifies list type construction with element types.
func TestListType(t *testing.T) {
	tests := []struct {
		name            string
		elementType     Type
		expectedElement TypeKind
	}{
		{"list of strings", StringType, String},
		{"list of ints", IntType, Int},
		{"list of any", AnyType, Any},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listType := NewListType(tt.elementType)
			if listType.Kind != List {
				t.Errorf("Kind = %v, want %v", listType.Kind, List)
			}
			if listType.ElementType == nil {
				t.Fatal("ElementType should not be nil for list type")
			}
			if listType.ElementType.Kind != tt.expectedElement {
				t.Errorf("ElementType.Kind = %v, want %v", listType.ElementType.Kind, tt.expectedElement)
			}
		})
	}
}

// TestMapType verifies map type construction with key and value types.
func TestMapType(t *testing.T) {
	tests := []struct {
		name          string
		keyType       Type
		valueType     Type
		expectedKey   TypeKind
		expectedValue TypeKind
	}{
		{"map string to string", StringType, StringType, String, String},
		{"map string to int", StringType, IntType, String, Int},
		{"map string to any", StringType, AnyType, String, Any},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapType := NewMapType(tt.keyType, tt.valueType)
			if mapType.Kind != Map {
				t.Errorf("Kind = %v, want %v", mapType.Kind, Map)
			}
			if mapType.KeyType == nil {
				t.Fatal("KeyType should not be nil for map type")
			}
			if mapType.ValueType == nil {
				t.Fatal("ValueType should not be nil for map type")
			}
			if mapType.KeyType.Kind != tt.expectedKey {
				t.Errorf("KeyType.Kind = %v, want %v", mapType.KeyType.Kind, tt.expectedKey)
			}
			if mapType.ValueType.Kind != tt.expectedValue {
				t.Errorf("ValueType.Kind = %v, want %v", mapType.ValueType.Kind, tt.expectedValue)
			}
		})
	}
}

// TestBlockRefType verifies block reference type construction.
func TestBlockRefType(t *testing.T) {
	tests := []struct {
		name              string
		blockType         string
		expectedBlockType string
	}{
		{"model ref", "model", "model"},
		{"agent ref", "agent", "agent"},
		{"tool ref", "tool", "tool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refType := NewBlockRefType(tt.blockType)
			if refType.Kind != BlockRef {
				t.Errorf("Kind = %v, want %v", refType.Kind, BlockRef)
			}
			if refType.BlockType != tt.expectedBlockType {
				t.Errorf("BlockType = %q, want %q", refType.BlockType, tt.expectedBlockType)
			}
		})
	}
}

// TestTypeEquals verifies type equality comparison.
func TestTypeEquals(t *testing.T) {
	tests := []struct {
		name     string
		a        Type
		b        Type
		expected bool
	}{
		{"same primitives", StringType, StringType, true},
		{"different primitives", StringType, IntType, false},
		{"list vs list same element", NewListType(StringType), NewListType(StringType), true},
		{"list vs list diff element", NewListType(StringType), NewListType(IntType), false},
		{"map vs map same", NewMapType(StringType, IntType), NewMapType(StringType, IntType), true},
		{"map vs map diff value", NewMapType(StringType, IntType), NewMapType(StringType, StringType), false},
		{"block ref same", NewBlockRefType("model"), NewBlockRefType("model"), true},
		{"block ref diff", NewBlockRefType("model"), NewBlockRefType("agent"), false},
		{"any equals any", AnyType, AnyType, true},
		{"primitive vs list", StringType, NewListType(StringType), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equals(tt.b); got != tt.expected {
				t.Errorf("Equals() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestTypeAnnotationToType verifies mapping from language-level type names to internal types.
func TestTypeAnnotationToType(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		expected   TypeKind
		ok         bool
	}{
		{"str annotation", "str", String, true},
		{"int annotation", "int", Int, true},
		{"float annotation", "float", Float, true},
		{"bool annotation", "bool", Bool, true},
		{"list annotation", "list", List, true},
		{"map annotation", "map", Map, true},
		{"any annotation", "any", Any, true},
		{"unknown annotation", "foobar", Any, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ, ok := TypeFromAnnotation(tt.annotation)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && typ.Kind != tt.expected {
				t.Errorf("Kind = %v, want %v", typ.Kind, tt.expected)
			}
		})
	}
}

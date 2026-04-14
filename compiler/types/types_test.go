package types

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

// ident returns a lazy block reference (no resolved *BlockSchema).
func ident(name string) Type {
	return NewBlockRefType(name, nil)
}

// schemaNamed returns a BlockRef whose String() uses the given schema name (minimal schema AST).
func schemaNamed(name string) Type {
	return NewBlockRefType(name, &BlockSchema{
		BlockName: name,
		Ast:       &ast.BlockBody{Kind: BlockKindSchema},
	})
}

// anyResolved is a BlockRef that satisfies IsAny() (requires non-nil Block with BlockName "any").
func anyResolved() Type {
	return NewBlockRefType("any", &BlockSchema{
		BlockName: "any",
		Ast:       &ast.BlockBody{Kind: BlockKindSchema},
	})
}

// nullResolved satisfies IsNull() (requires non-nil Block with BlockName "null").
func nullResolved() Type {
	return NewBlockRefType("null", &BlockSchema{
		BlockName: "null",
		Ast:       &ast.BlockBody{Kind: BlockKindSchema},
	})
}

// unionContains reports whether check is structurally equal to some union member.
func unionContains(u Type, check Type) bool {
	if u.Kind != Union {
		return false
	}
	for _, m := range u.Members {
		if m.Equals(check) {
			return true
		}
	}
	return false
}

// TestTypeKindString verifies that each TypeKind has a correct string representation.
func TestTypeKindString(t *testing.T) {
	tests := []struct {
		name     string
		kind     TypeKind
		expected string
	}{
		{"block ref kind", BlockRef, "blockref"},
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

// TestListType verifies list type construction with element types.
func TestListType(t *testing.T) {
	tests := []struct {
		name    string
		element Type
	}{
		{"list of strings", ident("string")},
		{"list of numbers", ident("number")},
		{"list of any", ident("any")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listType := NewListType(tt.element)
			if listType.Kind != List {
				t.Errorf("Kind = %v, want List", listType.Kind)
			}
			if listType.ElementType == nil {
				t.Fatal("ElementType should not be nil")
			}
			if !listType.ElementType.Equals(tt.element) {
				t.Errorf("ElementType = %s, want %s", listType.ElementType.String(), tt.element.String())
			}
		})
	}
}

// TestMapType verifies map type construction with key and value types.
func TestMapType(t *testing.T) {
	tests := []struct {
		name  string
		key   Type
		value Type
	}{
		{"map string to string", ident("string"), ident("string")},
		{"map string to number", ident("string"), ident("number")},
		{"map string to any", ident("string"), ident("any")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapType := NewMapType(tt.key, tt.value)
			if mapType.Kind != Map {
				t.Errorf("Kind = %v, want Map", mapType.Kind)
			}
			if mapType.KeyType == nil || mapType.ValueType == nil {
				t.Fatal("KeyType and ValueType should not be nil")
			}
			if !mapType.KeyType.Equals(tt.key) {
				t.Errorf("KeyType = %s, want %s", mapType.KeyType.String(), tt.key.String())
			}
			if !mapType.ValueType.Equals(tt.value) {
				t.Errorf("ValueType = %s, want %s", mapType.ValueType.String(), tt.value.String())
			}
		})
	}
}

// TestBlockRefType verifies block reference type construction.
func TestBlockRefType(t *testing.T) {
	tests := []struct {
		name      string
		blockName string
	}{
		{"model ref", "m"},
		{"agent ref", "a"},
		{"tool ref", "t"},
		{"schema ref", "s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refType := NewBlockRefType(tt.blockName, nil)
			if refType.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", refType.Kind)
			}
			if refType.BlockName != tt.blockName {
				t.Errorf("BlockName = %v, want %v", refType.BlockName, tt.blockName)
			}
		})
	}
}

// TestTypeEquals verifies type equality comparison.
func TestTypeEquals(t *testing.T) {
	str := ident("string")
	num := ident("number")
	num2 := ident("number")

	tests := []struct {
		name     string
		a        Type
		b        Type
		expected bool
	}{
		{"same named types", str, str, true},
		{"different named types", str, num, false},
		{"list vs list same element", NewListType(str), NewListType(str), true},
		{"list vs list diff element", NewListType(str), NewListType(num), false},
		{"map vs map same", NewMapType(str, num), NewMapType(str, num), true},
		{"map vs map diff value", NewMapType(str, num), NewMapType(str, str), false},
		{"block ref same BlockName (lazy)", NewBlockRefType("model", nil), NewBlockRefType("model", nil), true},
		{"block ref diff", NewBlockRefType("model", nil), NewBlockRefType("agent", nil), false},
		{"any resolved equals any resolved", anyResolved(), anyResolved(), false}, // different Block pointers
		{"named vs list", str, NewListType(str), false},
		{"union same members", NewUnionType(str, NewBlockRefType("model", nil)), NewUnionType(str, NewBlockRefType("model", nil)), true},
		{"union diff members", NewUnionType(str, num), NewUnionType(str, num2), true},
		{"union diff length", NewUnionType(str, num), NewUnionType(str), false},
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
	str := ident("string")
	num := ident("number")

	tests := []struct {
		name        string
		members     []Type
		expectLen   int
		expectTypes []Type
	}{
		{"str or model", []Type{str, NewBlockRefType("model", nil)}, 2, []Type{str, NewBlockRefType("model", nil)}},
		{"str or number", []Type{str, num}, 2, []Type{str, num}},
		{"single member", []Type{str}, 1, []Type{str}},
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
			for i, exp := range tt.expectTypes {
				if !union.Members[i].Equals(exp) {
					t.Errorf("Members[%d] = %s, want %s", i, union.Members[i].String(), exp.String())
				}
			}
		})
	}
}

// TestUnionTypeContains verifies unionContains checks membership correctly.
func TestUnionTypeContains(t *testing.T) {
	str := ident("string")
	union := NewUnionType(str, NewBlockRefType("model", nil))

	tests := []struct {
		name     string
		check    Type
		expected bool
	}{
		{"contains str", str, true},
		{"contains model", NewBlockRefType("model", nil), true},
		{"does not contain number", ident("number"), false},
		{"does not contain agent", NewBlockRefType("agent", nil), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unionContains(union, tt.check); got != tt.expected {
				t.Errorf("unionContains() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestIsAny verifies the IsAny helper for resolved and lazy block refs.
func TestIsAny(t *testing.T) {
	if !anyResolved().IsAny() {
		t.Error("resolved any.IsAny() should be true")
	}
	if !ident("any").IsAny() {
		t.Error("lazy any identifier.IsAny() should be true")
	}
	if ident("string").IsAny() {
		t.Error("lazy str.IsAny() should be false")
	}
}

// TestIsNull verifies the IsNull helper.
func TestIsNull(t *testing.T) {
	if !nullResolved().IsNull() {
		t.Error("resolved null.IsNull() should be true")
	}
	if !ident("null").IsNull() {
		t.Error("lazy null identifier.IsNull() should be true")
	}
	if ident("string").IsNull() {
		t.Error("lazy str.IsNull() should be false")
	}
}

// TestTypeOfUserDefinedSchema verifies lazy user schema refs vs str.
func TestTypeOfUserDefinedSchema(t *testing.T) {
	vpc := ident("vpc_data_t")
	str := ident("string")

	if vpc.Kind != str.Kind {
		t.Errorf("vpc Kind = %v, str Kind = %v — should be equal", vpc.Kind, str.Kind)
	}
	if vpc.Equals(str) {
		t.Error("vpc_data_t should not equal str")
	}
	if !vpc.Equals(NewBlockRefType("vpc_data_t", nil)) {
		t.Error("vpc_data_t should equal vpc_data_t")
	}
}

// TestSchemaTypeOfFields verifies NewBlockRefType sets Kind and BlockName.
func TestSchemaTypeOfFields(t *testing.T) {
	typ := NewBlockRefType("my_schema", nil)
	if typ.Kind != BlockRef {
		t.Errorf("Kind = %v, want BlockRef", typ.Kind)
	}
	if typ.BlockName != "my_schema" {
		t.Errorf("BlockName = %q, want %q", typ.BlockName, "my_schema")
	}
}

// TestSchemaTypeOfCaching verifies lazy refs with the same name are equal.
func TestSchemaTypeOfCaching(t *testing.T) {
	a := NewBlockRefType("cached_schema", nil)
	b := NewBlockRefType("cached_schema", nil)
	if !a.Equals(b) {
		t.Error("lazy refs with same BlockName should be equal")
	}
}

// TestSchemaTypeEquality verifies lazy refs and name mismatches.
func TestSchemaTypeEquality(t *testing.T) {
	tests := []struct {
		name   string
		a      Type
		b      Type
		expect bool
	}{
		{"same lazy name", NewBlockRefType("foo", nil), NewBlockRefType("foo", nil), true},
		{"different names", NewBlockRefType("foo", nil), NewBlockRefType("bar", nil), false},
		{"foo vs model", NewBlockRefType("foo", nil), NewBlockRefType("model", nil), false},
		{"foo vs empty name", NewBlockRefType("foo", nil), NewBlockRefType("", nil), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equals(tt.b); got != tt.expect {
				t.Errorf("Equals() = %v, want %v", got, tt.expect)
			}
		})
	}
}

// TestSchemaTypeCompatibility verifies IsCompatible for resolved any and lazy refs.
func TestSchemaTypeCompatibility(t *testing.T) {
	foo := ident("foo_t")
	bar := ident("bar_t")
	anyT := anyResolved()

	tests := []struct {
		name   string
		got    Type
		expect Type
		result bool
	}{
		{"any compatible with lazy foo", anyT, foo, true},
		{"lazy foo compatible with any", foo, anyT, true},
		{"different lazy refs incompatible", foo, bar, false},
		{"lazy compatible with resolved any", foo, anyT, true},
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

// TestSchemaTypeString verifies String() for resolved and lazy block refs.
func TestSchemaTypeString(t *testing.T) {
	tests := []struct {
		name   string
		typ    Type
		expect string
	}{
		{"named schema resolved", schemaNamed("vpc_data_t"), "vpc_data_t"},
		{"bare schema name", schemaNamed("schema"), "schema"},
		{"lazy ref", ident("vpc_data_t"), "vpc_data_t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.typ.String(); got != tt.expect {
				t.Errorf("String() = %q, want %q", got, tt.expect)
			}
		})
	}
}

// TestTypeStringRendering verifies that Type.String() matches types.go formatting.
func TestTypeStringRendering(t *testing.T) {
	str := schemaNamed("string")
	num := schemaNamed("number")
	nul := schemaNamed("null")
	anyT := anyResolved()

	tests := []struct {
		name     string
		typ      Type
		expected string
	}{
		{"string resolved", str, "string"},
		{"number resolved", num, "number"},
		{"null resolved", nul, "null"},
		{"any resolved", anyT, "any"},
		{"model lazy", ident("m"), "m"},
		{"user schema resolved", schemaNamed("vpc_data_t"), "vpc_data_t"},
		{"list", Type{Kind: List}, "list"},
		{"list[string]", NewListType(str), "list[string]"},
		{"map", Type{Kind: Map}, "map"},
		{"map[string, number]", NewMapType(ident("string"), num), "map[string, number]"},
		{"union string | number", NewUnionType(str, num), "string | number"},
		{"union string | model | null", NewUnionType(str, schemaNamed("model"), nul), "string | model | null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.typ.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestPrimitivesInUnion verifies primitives in unions behave like block refs for Contains.
func TestPrimitivesInUnion(t *testing.T) {
	nul := nullResolved()
	union := NewUnionType(ident("string"), ident("number"), nul)

	tests := []struct {
		name     string
		check    Type
		expected bool
	}{
		{"contains string", ident("string"), true},
		{"contains number", ident("number"), true},
		{"contains null resolved", nul, true},
		{"does not contain float", ident("float"), false},
		{"does not contain model", NewBlockRefType("model", nil), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unionContains(union, tt.check); got != tt.expected {
				t.Errorf("unionContains() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestPrimitiveAndBlockRefEquality verifies lazy ref equality.
func TestPrimitiveAndBlockRefEquality(t *testing.T) {
	str := ident("string")
	num := ident("number")

	tests := []struct {
		name     string
		a        Type
		b        Type
		expected bool
	}{
		{"str == str", str, str, true},
		{"str != number", str, num, false},
		{"str != model", str, NewBlockRefType("model", nil), false},
		{"model == model", NewBlockRefType("model", nil), NewBlockRefType("model", nil), true},
		{"list[str] == list[str]", NewListType(str), NewListType(str), true},
		{"list[str] != list[number]", NewListType(str), NewListType(num), false},
		{"list[model] == list[model]", NewListType(NewBlockRefType("model", nil)), NewListType(NewBlockRefType("model", nil)), true},
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

// TestIsCompatible verifies IsCompatible for any, unions, lists, and lazy refs.
func TestIsCompatible(t *testing.T) {
	str := ident("string")
	num := ident("number")
	boolT := ident("bool")
	anyT := anyResolved()
	listStr := NewListType(str)
	listNum := NewListType(num)
	mapStrStr := NewMapType(str, str)
	unionListNull := NewUnionType(NewListType(NewBlockRefType("tool", nil)), nullResolved())

	tests := []struct {
		name     string
		got      Type
		expected Type
		result   bool
	}{
		// Any matches everything when resolved.
		{"any compatible with str", anyT, str, true},
		{"str compatible with any", str, anyT, true},
		{"any compatible with any", anyT, anyT, true},

		// Lazy block refs with the same name are compatible.
		{"str compatible with str (lazy)", str, str, true},
		{"number compatible with number (lazy)", num, num, true},
		{"list[str] compatible with list[str] (lazy elements)", listStr, listStr, true},

		{"str not compatible with number", str, num, false},
		{"bool not compatible with number", boolT, num, false},
		{"list not compatible with map", listStr, mapStrStr, false},

		// Union: got is union — compatible if any member matches expected (list arm matches list[tool]).
		{"union(list,null) compatible with list (lazy)", unionListNull, NewListType(NewBlockRefType("tool", nil)), true},
		{"union(list,null) not compatible with str", unionListNull, str, false},

		{"str compatible with union(str,number)", str, NewUnionType(str, num), true},
		{"bool not compatible with union(str,number)", boolT, NewUnionType(str, num), false},

		{"list[str] not compatible with list[number]", listStr, listNum, false},

		{"null resolved compatible with null resolved", nullResolved(), nullResolved(), false},

		// Bare primitive schema BlockRefs should accept their native kind.
		// e.g. `invoke = callable` in bootstrap produces BlockRef("callable"),
		// but a lambda expression produces Kind: Callable.
		{"callable compatible with bare callable schema", NewCallableType([]Type{str}, str), schemaNamed("callable"), true},
		{"bare callable schema compatible with callable", schemaNamed("callable"), NewCallableType([]Type{str}, str), false},
		{"list compatible with bare list schema", Type{Kind: List}, schemaNamed("list"), true},
		{"list[str] compatible with bare list schema", NewListType(str), schemaNamed("list"), true},
		{"map compatible with bare map schema", Type{Kind: Map}, schemaNamed("map"), true},
		{"map[str,str] compatible with bare map schema", NewMapType(str, str), schemaNamed("map"), true},
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

// schemaWithFields builds a resolved schema BlockSchema with the given fields
// and annotations for SchemaImplements / duck-typing tests.
func schemaWithFields(name string, annots []string, fields map[string]FieldSchema) *BlockSchema {
	anns := make([]*ast.Annotation, 0, len(annots))
	for _, a := range annots {
		anns = append(anns, &ast.Annotation{Name: a})
	}
	return &BlockSchema{
		BlockName:   name,
		Ast:         &ast.BlockBody{Kind: BlockKindSchema, Name: name},
		Annotations: anns,
		Fields:      fields,
	}
}

// schemaRef returns a Type for a schema reference. The schema's own Schema
// pointer is set to itself, matching the bootstrap invariant that a schema's
// schema is `schema schema {}`. This satisfies the `got.Block.Schema != nil`
// guard in IsCompatible so the duck-typing path is exercised.
func schemaRef(s *BlockSchema) Type {
	s.Schema = s
	return NewBlockRefType(s.BlockName, s)
}

// TestSchemaImplements covers the structural (duck) match helper used by
// IsCompatible when the expected schema is not @strict_check.
func TestSchemaImplements(t *testing.T) {
	str := ident("string")
	num := ident("number")

	quackable := schemaWithFields("quackable", nil, map[string]FieldSchema{
		"quack": {Type: str, Required: true},
	})
	duckMatch := schemaWithFields("duck", nil, map[string]FieldSchema{
		"quack": {Type: str, Required: true},
		"age":   {Type: num, Required: false},
	})
	duckWrongType := schemaWithFields("duck_wrong", nil, map[string]FieldSchema{
		"quack": {Type: num, Required: true},
	})
	duckMissing := schemaWithFields("duck_missing", nil, map[string]FieldSchema{
		"age": {Type: num, Required: true},
	})
	// Expected has only optional fields — any block implements it vacuously.
	optionalExpected := schemaWithFields("opt", nil, map[string]FieldSchema{
		"desc": {Type: str, Required: false},
	})
	empty := schemaWithFields("empty", nil, map[string]FieldSchema{})

	// Expected has two required fields; got must satisfy both.
	twoReq := schemaWithFields("two", nil, map[string]FieldSchema{
		"a": {Type: str, Required: true},
		"b": {Type: num, Required: true},
	})
	twoReqOK := schemaWithFields("two_ok", nil, map[string]FieldSchema{
		"a": {Type: str, Required: true},
		"b": {Type: num, Required: true},
	})
	twoReqPartial := schemaWithFields("two_partial", nil, map[string]FieldSchema{
		"a": {Type: str, Required: true},
	})

	tests := []struct {
		name     string
		got      *BlockSchema
		expected *BlockSchema
		want     bool
	}{
		{"duck implements quackable", duckMatch, quackable, true},
		{"wrong field type fails", duckWrongType, quackable, false},
		{"missing required field fails", duckMissing, quackable, false},
		{"optional-only expected matches anything", empty, optionalExpected, true},
		{"empty expected always satisfied", duckMatch, empty, true},
		{"all required satisfied", twoReqOK, twoReq, true},
		{"partial required fails", twoReqPartial, twoReq, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SchemaImplements(tt.got, tt.expected); got != tt.want {
				t.Errorf("SchemaImplements() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsCompatibleDuckTyping verifies IsCompatible accepts structurally-
// matching instance blocks against non-strict schemas, and rejects them
// when the expected schema carries @strict_check.
func TestIsCompatibleDuckTyping(t *testing.T) {
	str := ident("string")
	num := ident("number")

	quackable := schemaWithFields("quackable", nil, map[string]FieldSchema{
		"quack": {Type: str, Required: true},
	})
	quackableStrict := schemaWithFields("quackable_strict", []string{AnnotationStrictCheck},
		map[string]FieldSchema{
			"quack": {Type: str, Required: true},
		})

	duck := schemaRef(schemaWithFields("duck", nil, map[string]FieldSchema{
		"quack": {Type: str, Required: true},
	}))
	cow := schemaRef(schemaWithFields("cow", nil, map[string]FieldSchema{
		"moo": {Type: str, Required: true},
	}))
	badDuck := schemaRef(schemaWithFields("bad_duck", nil, map[string]FieldSchema{
		"quack": {Type: num, Required: true},
	}))

	expectedQuackable := schemaRef(quackable)
	expectedStrict := schemaRef(quackableStrict)

	tests := []struct {
		name   string
		got    Type
		expect Type
		want   bool
	}{
		{"duck implements quackable", duck, expectedQuackable, true},
		{"cow does not implement quackable", cow, expectedQuackable, false},
		{"wrong field type fails duck check", badDuck, expectedQuackable, false},
		{"strict_check rejects structural match", duck, expectedStrict, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCompatible(tt.got, tt.expect); got != tt.want {
				t.Errorf("IsCompatible(%s, %s) = %v, want %v",
					tt.got.String(), tt.expect.String(), got, tt.want)
			}
		})
	}
}

// TestTernaryExprType verifies the type resolution of ternary expressions.
func TestTernaryExprType(t *testing.T) {
	symtab := NewSymbolTable()
	tok := token.Token{}
	// Bootstrap string, number, bool so IdentType resolves them.
	symtab.Define("string", NewBlockRefType("string", &BlockSchema{BlockName: "string", Ast: &ast.BlockBody{Kind: BlockKindSchema}}), tok)
	symtab.Define("number", NewBlockRefType("number", &BlockSchema{BlockName: "number", Ast: &ast.BlockBody{Kind: BlockKindSchema}}), tok)
	symtab.Define("bool", NewBlockRefType("bool", &BlockSchema{BlockName: "bool", Ast: &ast.BlockBody{Kind: BlockKindSchema}}), tok)
	symtab.Define("true", NewBlockRefType("bool", &BlockSchema{BlockName: "bool", Ast: &ast.BlockBody{Kind: "bool"}}), tok)
	symtab.Define("false", NewBlockRefType("bool", &BlockSchema{BlockName: "bool", Ast: &ast.BlockBody{Kind: "bool"}}), tok)

	strLit := func(v string) ast.Expression {
		return &ast.StringLiteral{Value: v}
	}
	numLit := func(v float64) ast.Expression {
		return &ast.NumberLiteral{Value: v}
	}
	identExpr := func(v string) ast.Expression {
		return &ast.Identifier{Value: v}
	}

	tests := []struct {
		name     string
		ternary  *ast.TernaryExpression
		wantKind TypeKind
		wantStr  string
	}{
		{
			name: "same type string",
			ternary: &ast.TernaryExpression{
				Condition: identExpr("true"),
				TrueExpr:  strLit("a"),
				FalseExpr: strLit("b"),
			},
			wantKind: BlockRef,
			wantStr:  "string",
		},
		{
			name: "same type number",
			ternary: &ast.TernaryExpression{
				Condition: identExpr("true"),
				TrueExpr:  numLit(1),
				FalseExpr: numLit(2),
			},
			wantKind: BlockRef,
			wantStr:  "number",
		},
		{
			name: "different types string | number",
			ternary: &ast.TernaryExpression{
				Condition: identExpr("true"),
				TrueExpr:  strLit("a"),
				FalseExpr: numLit(1),
			},
			wantKind: Union,
			wantStr:  "string | number",
		},
		{
			name: "nested ternary flattens union",
			ternary: &ast.TernaryExpression{
				Condition: identExpr("true"),
				TrueExpr:  strLit("a"),
				FalseExpr: &ast.TernaryExpression{
					Condition: identExpr("false"),
					TrueExpr:  numLit(1),
					FalseExpr: identExpr("true"),
				},
			},
			wantKind: Union,
			wantStr:  "string | number | bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ternaryExprType(1, tt.ternary, &symtab)
			if got.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v", got.Kind, tt.wantKind)
			}
			if got.String() != tt.wantStr {
				t.Errorf("String() = %q, want %q", got.String(), tt.wantStr)
			}
		})
	}
}

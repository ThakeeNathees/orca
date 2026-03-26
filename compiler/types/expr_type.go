package types

import "github.com/thakee/orca/compiler/ast"

// ExprType returns the type of an expression. Uses the symbol table to
// resolve identifiers and member access. If symbols is nil, identifiers
// return any.
func ExprType(expr ast.Expression, symbols *SymbolTable) Type {
	switch e := expr.(type) {
	case *ast.StringLiteral:
		return TypeOf("str")
	case *ast.IntegerLiteral:
		return TypeOf("int")
	case *ast.FloatLiteral:
		return TypeOf("float")
	case *ast.BooleanLiteral:
		return TypeOf("bool")
	case *ast.NullLiteral:
		return TypeOf("null")
	case *ast.ListLiteral:
		return listLiteralType(e, symbols)
	case *ast.MapLiteral:
		return mapLiteralType(e, symbols)
	case *ast.Identifier:
		return identType(e, symbols)
	case *ast.MemberAccess:
		return memberAccessType(e, symbols)
	case *ast.SchemaExpression:
		return TypeOf("schema")
	case *ast.BinaryExpression:
		// TODO: infer result type from operator and operand types.
		return TypeOf("any")
	case *ast.Subscription:
		return subscriptResultType(ExprType(e.Object, symbols))
	case *ast.CallExpression:
		// TODO: resolve return type from the callee's type.
		return TypeOf("any")
	default:
		return TypeOf("any")
	}
}

// identType resolves an identifier's type from the symbol table.
// Returns the block reference type if found, any otherwise.
func identType(ident *ast.Identifier, symbols *SymbolTable) Type {
	if symbols == nil {
		return TypeOf("any")
	}
	if typ, ok := symbols.Lookup(ident.Value); ok {
		return typ
	}
	return TypeOf("any")
}

// memberAccessType resolves the type of a member access expression
// (e.g. gpt4.model_name). Looks up the object's type, then finds
// the member's type in the corresponding block schema.
func memberAccessType(ma *ast.MemberAccess, symbols *SymbolTable) Type {
	// Incomplete member access (e.g. "gpt4." while typing).
	if ma.Member == "" {
		return TypeOf("any")
	}

	objType := ExprType(ma.Object, symbols)
	if objType.Kind != BlockRef {
		return TypeOf("any")
	}

	schema, ok := GetBlockSchema(string(objType.BlockType))
	if !ok {
		return TypeOf("any")
	}

	field, ok := schema.Fields[ma.Member]
	if !ok {
		return TypeOf("any")
	}

	return field.Type
}

// subscriptResultType returns the element/value type when subscripting a type.
// For list[T] returns T, for map[K,V] returns V, for unions checks members.
func subscriptResultType(t Type) Type {
	switch t.Kind {
	case List:
		if t.ElementType != nil {
			return *t.ElementType
		}
		return TypeOf("any")
	case Map:
		if t.ValueType != nil {
			return *t.ValueType
		}
		return TypeOf("any")
	case Union:
		// Find the subscriptable member and return its result type.
		for _, m := range t.Members {
			if m.Kind == List || m.Kind == Map {
				return subscriptResultType(m)
			}
		}
		return TypeOf("any")
	default:
		return TypeOf("any")
	}
}

// mapLiteralType infers the type of a map literal. Keys are always strings.
// If all values have the same type, returns map[T]. Otherwise returns
// an untyped map.
func mapLiteralType(m *ast.MapLiteral, symbols *SymbolTable) Type {
	if len(m.Entries) == 0 {
		return Type{Kind: Map}
	}

	first := ExprType(m.Entries[0].Value, symbols)
	for _, entry := range m.Entries[1:] {
		if !ExprType(entry.Value, symbols).Equals(first) {
			return Type{Kind: Map}
		}
	}
	return NewMapType(TypeOf("str"), first)
}

// listLiteralType infers the type of a list literal. If all elements
// have the same type, returns list[T]. Otherwise returns an untyped list.
func listLiteralType(list *ast.ListLiteral, symbols *SymbolTable) Type {
	if len(list.Elements) == 0 {
		return Type{Kind: List}
	}

	first := ExprType(list.Elements[0], symbols)
	for _, elem := range list.Elements[1:] {
		if !ExprType(elem, symbols).Equals(first) {
			return Type{Kind: List}
		}
	}
	return NewListType(first)
}

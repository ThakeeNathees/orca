package types

import "github.com/thakee/orca/compiler/ast"

// ExprType returns the type of an expression. Uses the symbol table to
// resolve identifiers and member access. If symbols is nil, identifiers
// return Any.
func ExprType(expr ast.Expression, symbols *SymbolTable) Type {
	switch e := expr.(type) {
	case *ast.StringLiteral:
		return StringType
	case *ast.IntegerLiteral:
		return IntType
	case *ast.FloatLiteral:
		return FloatType
	case *ast.BooleanLiteral:
		return BoolType
	case *ast.ListLiteral:
		return listLiteralType(e, symbols)
	case *ast.MapLiteral:
		return mapLiteralType(e, symbols)
	case *ast.Identifier:
		return identType(e, symbols)
	case *ast.MemberAccess:
		return memberAccessType(e, symbols)
	case *ast.BinaryExpression:
		// TODO: infer result type from operator and operand types.
		return AnyType
	case *ast.Subscription:
		// TODO: resolve element type from the object's type.
		return AnyType
	case *ast.CallExpression:
		// TODO: resolve return type from the callee's type.
		return AnyType
	default:
		return AnyType
	}
}

// identType resolves an identifier's type from the symbol table.
// Returns the block reference type if found, Any otherwise.
func identType(ident *ast.Identifier, symbols *SymbolTable) Type {
	if symbols == nil {
		return AnyType
	}
	if typ, ok := symbols.Lookup(ident.Value); ok {
		return typ
	}
	return AnyType
}

// memberAccessType resolves the type of a member access expression
// (e.g. gpt4.model_name). Looks up the object's type, then finds
// the member's type in the corresponding block schema.
func memberAccessType(ma *ast.MemberAccess, symbols *SymbolTable) Type {
	objType := ExprType(ma.Object, symbols)
	if objType.Kind != BlockRef {
		return AnyType
	}

	schema, ok := GetBlockSchema(string(objType.BlockType))
	if !ok {
		return AnyType
	}

	field, ok := schema.Fields[ma.Member]
	if !ok {
		return AnyType
	}

	return field.Type
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
	return NewMapType(StringType, first)
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

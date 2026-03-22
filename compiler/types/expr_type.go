package types

import "github.com/thakee/orca/compiler/ast"

// ExprType returns the type of an expression based on its AST node.
// For literals, the type is known statically. For identifiers and complex
// expressions, type resolution requires scope/reference information that
// is not yet available — these return Any for now.
func ExprType(expr ast.Expression) Type {
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
		return listLiteralType(e)
	case *ast.MapLiteral:
		return mapLiteralType(e)
	case *ast.Identifier:
		// TODO: resolve identifier type from scope (block references, etc.)
		return AnyType
	case *ast.BinaryExpression:
		// TODO: infer result type from operator and operand types.
		return AnyType
	case *ast.MemberAccess:
		// TODO: resolve member type from the object's type.
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

// mapLiteralType infers the type of a map literal. Keys are always strings.
// If all values have the same type, returns map[T]. Otherwise returns
// an untyped map.
func mapLiteralType(m *ast.MapLiteral) Type {
	if len(m.Entries) == 0 {
		return Type{Kind: Map}
	}

	first := ExprType(m.Entries[0].Value)
	for _, entry := range m.Entries[1:] {
		if !ExprType(entry.Value).Equals(first) {
			return Type{Kind: Map}
		}
	}
	return NewMapType(StringType, first)
}

// listLiteralType infers the type of a list literal. If all elements
// have the same type, returns list[T]. Otherwise returns an untyped list.
func listLiteralType(list *ast.ListLiteral) Type {
	if len(list.Elements) == 0 {
		return Type{Kind: List}
	}

	first := ExprType(list.Elements[0])
	for _, elem := range list.Elements[1:] {
		if !ExprType(elem).Equals(first) {
			return Type{Kind: List}
		}
	}
	return NewListType(first)
}

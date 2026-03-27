package types

import (
	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

// ExprType returns the type of an expression. Uses the symbol table to
// resolve identifiers and member access. If symbols is nil, identifiers
// return any.
func ExprType(expr ast.Expression, symbols *SymbolTable) Type {
	switch e := expr.(type) {
	case *ast.StringLiteral:
		return TypeOf(token.BlockStr)
	case *ast.IntegerLiteral:
		return TypeOf(token.BlockInt)
	case *ast.FloatLiteral:
		return TypeOf(token.BlockFloat)
	case *ast.BooleanLiteral:
		return TypeOf(token.BlockBool)
	case *ast.NullLiteral:
		return TypeOf(token.BlockNull)
	case *ast.ListLiteral:
		return listLiteralType(e, symbols)
	case *ast.MapLiteral:
		return mapLiteralType(e, symbols)
	case *ast.Identifier:
		return identType(e, symbols)
	case *ast.MemberAccess:
		return memberAccessType(e, symbols)
	case *ast.SchemaExpression:
		return TypeOf(token.BlockSchema)
	case *ast.BinaryExpression:
		// TODO: infer result type from operator and operand types.
		return TypeOf(token.BlockAny)
	case *ast.Subscription:
		return subscriptResultType(ExprType(e.Object, symbols))
	case *ast.CallExpression:
		// TODO: resolve return type from the callee's type.
		return TypeOf(token.BlockAny)
	default:
		return TypeOf(token.BlockAny)
	}
}

// identType resolves an identifier's type from the symbol table.
// Returns the block reference type if found, any otherwise.
func identType(ident *ast.Identifier, symbols *SymbolTable) Type {
	if symbols == nil {
		return TypeOf(token.BlockAny)
	}
	if typ, ok := symbols.Lookup(ident.Value); ok {
		return typ
	}
	return TypeOf(token.BlockAny)
}

// memberAccessType resolves the type of a member access expression
// (e.g. gpt4.model_name). Looks up the object's type, then finds
// the member's type in the corresponding block schema.
func memberAccessType(ma *ast.MemberAccess, symbols *SymbolTable) Type {
	// Incomplete member access (e.g. "gpt4." while typing).
	if ma.Member == "" {
		return TypeOf(token.BlockAny)
	}

	objType := ExprType(ma.Object, symbols)
	if objType.Kind != BlockRef {
		return TypeOf(token.BlockAny)
	}

	schema, ok := LookupBlockSchema(objType)
	if !ok {
		return TypeOf(token.BlockAny)
	}

	field, ok := schema.Fields[ma.Member]
	if !ok {
		return TypeOf(token.BlockAny)
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
		return TypeOf(token.BlockAny)
	case Map:
		if t.ValueType != nil {
			return *t.ValueType
		}
		return TypeOf(token.BlockAny)
	case Union:
		// Find the subscriptable member and return its result type.
		for _, m := range t.Members {
			if m.Kind == List || m.Kind == Map {
				return subscriptResultType(m)
			}
		}
		return TypeOf(token.BlockAny)
	default:
		return TypeOf(token.BlockAny)
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
	return NewMapType(TypeOf(token.BlockStr), first)
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

package types

// FIXME: This should moved to analyzer.

import (
	"fmt"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

// ExprType returns the type of an expression. Uses the symbol table to
// resolve identifiers and member access. When symbols is nil (bootstrap
// mode), identifiers are resolved as type names — this is used when
// loading builtins.oc where the RHS of assignments are type declarations.
func ExprType(expr ast.Expression, symbols *SymbolTable) Type {
	switch e := expr.(type) {
	// FIXME: Move the "str", "number", "bool", "null" to constants.go file.
	case *ast.StringLiteral:
		return IdentType("str", symbols)
	case *ast.NumberLiteral:
		return IdentType("number", symbols)
	case *ast.BooleanLiteral:
		return IdentType("bool", symbols)
	case *ast.NullLiteral:
		return IdentType("null", symbols)
	case *ast.ListLiteral:
		return listLiteralType(e, symbols)
	case *ast.MapLiteral:
		return mapLiteralType(e, symbols)
	case *ast.Identifier:
		return IdentType(e.Value, symbols)
	case *ast.MemberAccess:
		return memberAccessType(e, symbols)
	case *ast.BlockExpression:
		return blockExprType(e, symbols)
	case *ast.BinaryExpression:
		return binaryExprType(e, symbols)
	case *ast.Subscription:
		return subscriptionType(e, symbols)
	case *ast.CallExpression:
		// TODO: resolve return type from the callee's type.
		return anyType(symbols)
	default:
		return anyType(symbols)
	}
}

// blockExprType returns the type of an inline block expression.
// For schema blocks, registers the inline schema under a synthetic name
// so member access can resolve through it.
func blockExprType(e *ast.BlockExpression, symtab *SymbolTable) Type {
	name := fmt.Sprintf("__anon_%d", inlineCounter.Add(1))
	schema := NewBlockSchema(nil, name, &e.BlockBody, symtab)
	return NewBlockRefType(name, &schema)
}

// binaryExprType resolves the type of a binary expression. Pipe operators
// produce union types (e.g. str | null). Arithmetic operators (+, -, *, /)
// apply numeric promotion rules and string concatenation. Other operators
// return any.
func binaryExprType(e *ast.BinaryExpression, symbols *SymbolTable) Type {
	switch e.Operator.Type {
	case token.PIPE:
		// When both operands are schema types, | constructs a union type (e.g. str | null).
		// TODO: when both operands are numeric (int | int, int | float, etc.), | should be
		// treated as bitwise OR — this is not yet implemented.
		members := flattenUnionTypes(e, symbols)
		if len(members) == 0 {
			return anyType(symbols)
		}
		return NewUnionType(members...)
	case token.PLUS, token.MINUS, token.STAR, token.SLASH:
		return arithmeticResultType(e, symbols)
	default:
		return anyType(symbols)
	}
}

// arithmeticResultType infers the result type of an arithmetic binary expression.
// Rules:
//   - str + str → str  (string concatenation, PLUS only)
//   - int op int → int
//   - float op float → float
//   - int op float / float op int → float  (numeric widening)
//   - otherwise → any
func arithmeticResultType(e *ast.BinaryExpression, symbols *SymbolTable) Type {
	left := ExprType(e.Left, symbols)
	right := ExprType(e.Right, symbols)

	// String concatenation: str + str → str.
	isLeftStr := IsCompatible(left, IdentType("str", symbols))
	isRightStr := IsCompatible(right, IdentType("str", symbols))
	if e.Operator.Type == token.PLUS && isLeftStr && isRightStr {
		return IdentType("str", symbols)
	}

	// <number> op <number> → <number>.
	isLeftNum := IsCompatible(left, IdentType("number", symbols))
	isRightNum := IsCompatible(right, IdentType("number", symbols))
	if isLeftNum && isRightNum {
		return IdentType("number", symbols)
	}

	return anyType(symbols)
}

// flattenUnionTypes recursively collects all members of a pipe-separated
// union expression into a flat slice.
func flattenUnionTypes(expr ast.Expression, symbols *SymbolTable) []Type {
	switch e := expr.(type) {
	case *ast.BinaryExpression:
		if e.Operator.Type != token.PIPE {
			return []Type{ExprType(expr, symbols)}
		}
		left := flattenUnionTypes(e.Left, symbols)
		right := flattenUnionTypes(e.Right, symbols)
		return append(left, right...)
	default:
		return []Type{ExprType(expr, symbols)}
	}
}

// IdentType resolves an identifier's type. With a symbol table, looks up the
// identifier. Without one (bootstrap mode), resolves as a type name — block
// keywords become block references, everything else is a schema type.
func IdentType(name string, symtab *SymbolTable) Type {
	if symtab != nil {
		if ty, ok := symtab.Lookup(name); ok {
			return ty
		}
	}
	// Not found in symbol table, we set the block schema to nil
	// and the analyzer will either resolve or report an error.
	// because the block may defined after the current one.
	return NewBlockRefType(name, nil)
}

// subscriptionType returns the type of a subscription expression.
// In bootstrap mode (symbols == nil), first tries to resolve parameterized
// types like list[tool] or map[str], then falls back to subscript result
// type inference. With symbols, always infers from the object type.
func subscriptionType(e *ast.Subscription, symtab *SymbolTable) Type {
	// Bootstrap: try parameterized type like list[tool] or map[str].
	if baseIdent, ok := e.Object.(*ast.Identifier); ok {
		elemType := ExprType(e.Index, symtab)
		switch baseIdent.Value {
		case "list":
			return NewListType(elemType)
		case "map":
			// NOTE: Map keys are always "str" but we set anyways maybe
			// in the future we support other key types.
			return NewMapType(IdentType("str", symtab), elemType)
		}
	}
	return subscriptResultType(ExprType(e.Object, symtab), symtab)
}

// memberAccessType resolves the type of a member access expression
// (e.g. gpt4.model_name). Looks up the object's type, then finds
// the member's type in the corresponding block schema.
func memberAccessType(ma *ast.MemberAccess, symtab *SymbolTable) Type {
	// Incomplete member access (e.g. "gpt4." while typing).
	if ma.Member == "" {
		return anyType(symtab)
	}

	objType := ExprType(ma.Object, symtab)

	// TODO: Handle other kinds, Best to remove list, and map from kinds.
	if objType.Kind != BlockRef {
		return anyType(symtab)
	}

	// If the block is not resolved, try to resolve it from the symbol table.
	if objType.Block == nil {
		if ty, ok := symtab.Lookup(objType.BlockName); ok {
			objType.Block = ty.Block
		}
	}

	if objType.Block == nil {
		return anyType(symtab)
	}

	field, ok := objType.Block.Fields[ma.Member]
	if !ok {
		return anyType(symtab)
	}

	return field.Type
}

// subscriptResultType returns the element/value type when subscripting a type.
// For list[T] returns T, for map[K,V] returns V, for unions checks members.
func subscriptResultType(t Type, symtab *SymbolTable) Type {
	switch t.Kind {
	case List:
		if t.ElementType != nil {
			return *t.ElementType
		}
		return anyType(symtab)
	case Map:
		if t.ValueType != nil {
			return *t.ValueType
		}
		return anyType(symtab)
	case Union:
		// Find the subscriptable member and return its result type.
		for _, m := range t.Members {
			if m.Kind == List || m.Kind == Map {
				return subscriptResultType(m, symtab)
			}
		}
		return anyType(symtab)
	default:
		return anyType(symtab)
	}
}

// mapLiteralType infers the type of a map literal. Keys are always strings.
// If all values have the same type, returns map[T]. Otherwise returns
// an untyped map.
func mapLiteralType(m *ast.MapLiteral, symtab *SymbolTable) Type {
	if len(m.Entries) == 0 {
		return Type{Kind: Map}
	}

	// TODO: Check if the first element's type is compatible with
	// all the other elements' types. in that case, that can be the
	// map value type.
	//
	// first := ExprType(m.Entries[0].Value, symbols)
	// for _, entry := range m.Entries[1:] {
	// }

	return NewMapType(IdentType("str", symtab), anyType(symtab))
}

// listLiteralType infers the type of a list literal. If all elements
// have the same type, returns list[T]. Otherwise returns an untyped list.
func listLiteralType(list *ast.ListLiteral, symbols *SymbolTable) Type {
	if len(list.Elements) == 0 {
		return Type{Kind: List}
	}

	// TODO: Check if the first element's type is compatible with
	// first := ExprType(list.Elements[0], symbols)
	// for _, elem := range list.Elements[1:] {
	// 	if !ExprType(elem, symbols).Equals(first) {
	// 		return Type{Kind: List}
	// 	}
	// }
	// return NewListType(first)

	return Type{Kind: List}
}

func anyType(symtab *SymbolTable) Type {
	return IdentType("any", symtab)
}

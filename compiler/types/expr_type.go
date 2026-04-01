package types

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
	case *ast.StringLiteral:
		return Str()
	case *ast.IntegerLiteral:
		return Int()
	case *ast.FloatLiteral:
		return Float()
	case *ast.BooleanLiteral:
		return Bool()
	case *ast.NullLiteral:
		return Null()
	case *ast.ListLiteral:
		return listLiteralType(e, symbols)
	case *ast.MapLiteral:
		return mapLiteralType(e, symbols)
	case *ast.Identifier:
		return identType(e, symbols)
	case *ast.MemberAccess:
		return memberAccessType(e, symbols)
	case *ast.BlockExpression:
		return blockExprType(e)
	case *ast.BinaryExpression:
		return binaryExprType(e, symbols)
	case *ast.Subscription:
		return subscriptionType(e, symbols)
	case *ast.CallExpression:
		// TODO: resolve return type from the callee's type.
		return Any()
	default:
		return Any()
	}
}

// identType resolves an identifier's type. With a symbol table, looks up the
// identifier. Without one (bootstrap mode), resolves as a type name — block
// keywords become block references, everything else is a schema type.
func identType(ident *ast.Identifier, symbols *SymbolTable) Type {
	if symbols != nil {
		if typ, ok := symbols.Lookup(ident.Value); ok {
			return typ
		}
	}
	// Bootstrap / fallback: resolve as a type name.
	return resolveIdentAsType(ident.Value)
}

// resolveIdentAsType maps an identifier name to an internal Type.
// Block type names resolve via TokenTypeToBlockKind; primitives and
// user-defined names resolve as schema types.
func resolveIdentAsType(name string) Type {
	tokType := token.LookupIdent(name)
	if kind, ok := token.TokenTypeToBlockKind(tokType); ok {
		return NewBlockRefType(kind)
	}
	return CreateSchema(name)
}

// blockExprType returns the type of an inline block expression.
// For schema blocks, registers the inline schema under a synthetic name
// so member access can resolve through it.
func blockExprType(e *ast.BlockExpression) Type {
	if e.Kind == token.BlockSchema {
		schema, err := SchemaFromAssignments(e.Assignments)
		if err != nil {
			return TypeOf(token.BlockSchema)
		}
		name := fmt.Sprintf("__anon_%d", inlineCounter.Add(1))
		RegisterSchema(name, schema)
		return CreateSchema(name)
	}
	return TypeOf(e.Kind)
}

// binaryExprType resolves the type of a binary expression. Pipe operators
// produce union types (e.g. str | null). Arithmetic operators (+, -, *, /)
// apply numeric promotion rules and string concatenation. Other operators
// return any.
func binaryExprType(e *ast.BinaryExpression, symbols *SymbolTable) Type {
	switch e.Operator.Type {
	case token.PIPE:
		members := flattenUnionTypes(e, symbols)
		if len(members) == 0 {
			return Any()
		}
		return NewUnionType(members...)
	case token.PLUS, token.MINUS, token.STAR, token.SLASH:
		return arithmeticResultType(e, symbols)
	default:
		return Any()
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
	if e.Operator.Type == token.PLUS && left.Equals(Str()) && right.Equals(Str()) {
		return Str()
	}

	// Numeric promotions.
	leftInt := left.Equals(Int())
	leftFloat := left.Equals(Float())
	rightInt := right.Equals(Int())
	rightFloat := right.Equals(Float())

	switch {
	case leftInt && rightInt:
		return Int()
	case leftFloat && rightFloat:
		return Float()
	case (leftInt && rightFloat) || (leftFloat && rightInt):
		return Float()
	default:
		return Any()
	}
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

// subscriptionType returns the type of a subscription expression.
// In bootstrap mode (symbols == nil), first tries to resolve parameterized
// types like list[tool] or map[str], then falls back to subscript result
// type inference. With symbols, always infers from the object type.
func subscriptionType(e *ast.Subscription, symbols *SymbolTable) Type {
	if symbols == nil {
		// Bootstrap: try parameterized type like list[tool] or map[str].
		if baseIdent, ok := e.Object.(*ast.Identifier); ok {
			elemType := ExprType(e.Index, nil)
			switch baseIdent.Value {
			case "list":
				return NewListType(elemType)
			case "map":
				return NewMapType(Str(), elemType)
			}
		}
	}
	return subscriptResultType(ExprType(e.Object, symbols))
}

// memberAccessType resolves the type of a member access expression
// (e.g. gpt4.model_name). Looks up the object's type, then finds
// the member's type in the corresponding block schema.
func memberAccessType(ma *ast.MemberAccess, symbols *SymbolTable) Type {
	// Incomplete member access (e.g. "gpt4." while typing).
	if ma.Member == "" {
		return Any()
	}

	objType := ExprType(ma.Object, symbols)
	if objType.Kind != BlockRef {
		return Any()
	}

	schema, ok := LookupBlockSchema(objType)
	if !ok {
		return Any()
	}

	field, ok := schema.Fields[ma.Member]
	if !ok {
		return Any()
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
		return Any()
	case Map:
		if t.ValueType != nil {
			return *t.ValueType
		}
		return Any()
	case Union:
		// Find the subscriptable member and return its result type.
		for _, m := range t.Members {
			if m.Kind == List || m.Kind == Map {
				return subscriptResultType(m)
			}
		}
		return Any()
	default:
		return Any()
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
	return NewMapType(Str(), first)
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

package types

// FIXME: This should moved to analyzer.

import (
	"fmt"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

// Return the schema type of the given expression.
//
// i.e.
// if expr is true, bool true {} is the block where true is defined
// so i'tll return Type(BlockRef(bool true {}))
//
// writer is agent writer {} so SchemaFromExpr(writer) will return
// Type(BlockRef(agent writer {}))
//
// This is a companion function to SchemaTypeFromExpr(). where the return
// type of this function can be `anyKind anyValue {}` but SchemaTypeFromExpr()
// will always be `schema <something> {}`
func BlockSchemaTypeOfExpr(expr ast.Expression, symbols *SymbolTable) Type {
	return schemaFromExprWithDepth(0, expr, symbols)
}

// Return the schema type of the expression.
//
// i.e.
// if expr is true, bool true {} is the block where true is defined
// and this functino will return the schema type of bool that is
// `schema bool {}`
//
// writer is agent writer {} so SchemaTypeOfExpr(writer) will return
// the schema type of agent that is `schema agent {}`
//
// The return value will be schema <something> {}

func SchemaTypeFromExpr(expr ast.Expression, symbols *SymbolTable) Type {
	return schemaFromExprWithDepth(1, expr, symbols)
}

// Schema depth is how deep it needs to go and get the schema.
//
//	schema agent {}
//	agnet writer {}
//
// expr is `writer`:
// if depth is 0, it'll return `agent writer {}` as Type()
// if depth is 1, it'll return `schema agent {}` as Type()
//
// In theory we can think of multi depth like this:
//
//	schema foo {}
//	foo bar {}
//	bar baz {}
//
// if expr is `baz`,
//
//	depth 0 -> `foo baz {}`
//	depth 1 -> `schema foo {}`
//	depth 2 -> `schema schema {}`
func schemaFromExprWithDepth(depth int, expr ast.Expression, symbols *SymbolTable) Type {
	switch e := expr.(type) {

	// NOTE: actually bool true and false are currently identifiers and defined
	// in the bootstrap.oc file. as `bool true {}` and `bool false {}`
	//
	// Ideally we want the string, number to be the same like
	// `str "foo"` and `number 42` to be the same (consecptually), so
	// instead of doing the (depth-1) + search. That is
	//
	// number 42 {}
	// Expr is `42`, depth is 1 (what's the schema of 42? (schema number {}))
	// so what we're doing is what's the block schema (depth-1) of "number"
	//
	// The question, whats the block schema of 42? (depth 0) doesnt make any
	// sense, however in theory it should work, so we dynamically define something
	// like this (maybe not but it's paper worthy to point out). `number 42 {}`
	//
	// FIXME: Move the "str", "number", "null" to constants.go file.
	case *ast.StringLiteral:
		return IdentType(depth-1, "str", symbols)
	case *ast.NumberLiteral:
		return IdentType(depth-1, "number", symbols)
	case *ast.NullLiteral:
		return IdentType(depth-1, "null", symbols)

	case *ast.ListLiteral:
		return listLiteralType(depth, e, symbols)
	case *ast.MapLiteral:
		return mapLiteralType(depth, e, symbols)
	case *ast.Identifier:
		return IdentType(depth, e.Value, symbols)

	case *ast.MemberAccess:
		return memberAccessType(depth, e, symbols)
	case *ast.Subscription:
		return subscriptionType(depth, e, symbols)
	case *ast.CallExpression:
		// TODO: resolve return type from the callee's type.
		return anyType(symbols)
	case *ast.BinaryExpression:
		return binaryExprType(depth, e, symbols)
	case *ast.BlockExpression:
		return blockExprType(depth, e, symbols)
	default:
		return anyType(symbols)
	}
}

// blockExprType returns the type of an inline block expression.
// For schema blocks, registers the inline schema under a synthetic name
// so member access can resolve through it.
func blockExprType(depth int, e *ast.BlockExpression, symtab *SymbolTable) Type {

	if e.BlockNameAnon == "" {
		e.BlockNameAnon = fmt.Sprintf("__anon_%d", inlineCounter.Add(1))
	}

	// TODO: The depth parameter is not used here but it should be.

	// NOTE:
	//
	//	schema foo {}
	//	foo bar {}
	//	ExprType(bar) -> `schema foo {}` and not `foo bar {}`
	//
	// That means
	//
	//	 schema str {}
	//	 str "bar" {}
	//	 ExprType("bar") -> `schema str {}` and not `str "bar" {}`
	//
	//   schema agent {}
	//   agent writer {}
	//   ExprType(writer) -> `schema agent {}` and not `agent writer {}`
	//
	// IMPORTANT:
	//
	// agent writer {  model = model { ... } }
	// here we have a block expression that is model { ... }
	// Thats equivalent to:
	//
	//   schema model { ... }
	//   model __anon_1 { ... }
	//
	// Now in the symbol table "__anon_1" -> Type(BlockRef(model __anon_1 { ... }))
	// BUT, ExprType("__anon_1") -> `schema model { ... }` and not `model __anon_1 { ... }`
	//
	refBlock := NewBlockSchema(nil, e.BlockNameAnon, &e.BlockBody, symtab)
	ty := NewBlockRefType(e.BlockNameAnon, &refBlock)
	symtab.Define(e.BlockNameAnon, ty, e.Start())

	return NewBlockRefType(e.BlockBody.Kind, nil)
}

// binaryExprType resolves the type of a binary expression. Pipe operators
// produce union types (e.g. str | null). Arithmetic operators (+, -, *, /)
// apply numeric promotion rules and string concatenation. Other operators
// return any.
func binaryExprType(depth int, e *ast.BinaryExpression, symbols *SymbolTable) Type {
	switch e.Operator.Type {
	case token.PIPE:
		// When both operands are schema types, | constructs a union type (e.g. str | null).
		// TODO: when both operands are numeric (int | int, int | float, etc.), | should be
		// treated as bitwise OR — this is not yet implemented.
		members := flattenUnionTypes(depth, e, symbols)
		if len(members) == 0 {
			return anyType(symbols)
		}
		return NewUnionType(members...)
	case token.PLUS, token.MINUS, token.STAR, token.SLASH:
		return arithmeticResultType(depth, e, symbols)
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
func arithmeticResultType(depth int, e *ast.BinaryExpression, symbols *SymbolTable) Type {
	left := schemaFromExprWithDepth(depth, e.Left, symbols)
	right := schemaFromExprWithDepth(depth, e.Right, symbols)

	// String concatenation: str + str → str.
	isLeftStr := IsCompatible(left, IdentType(0, "str", symbols))
	isRightStr := IsCompatible(right, IdentType(0, "str", symbols))
	if e.Operator.Type == token.PLUS && isLeftStr && isRightStr {
		return IdentType(0, "str", symbols)
	}

	// <number> op <number> → <number>.
	isLeftNum := IsCompatible(left, IdentType(0, "number", symbols))
	isRightNum := IsCompatible(right, IdentType(0, "number", symbols))
	if isLeftNum && isRightNum {
		return IdentType(0, "number", symbols)
	}

	return anyType(symbols)
}

// flattenUnionTypes recursively collects all members of a pipe-separated
// union expression into a flat slice.
func flattenUnionTypes(depth int, expr ast.Expression, symbols *SymbolTable) []Type {
	switch e := expr.(type) {
	case *ast.BinaryExpression:
		if e.Operator.Type != token.PIPE {
			return []Type{schemaFromExprWithDepth(depth, expr, symbols)}
		}
		left := flattenUnionTypes(depth, e.Left, symbols)
		right := flattenUnionTypes(depth, e.Right, symbols)
		return append(left, right...)
	default:
		return []Type{schemaFromExprWithDepth(depth, expr, symbols)}
	}
}

// IdentType resolves an identifier's type. With a symbol table, looks up the
// identifier. Without one (bootstrap mode), resolves as a type name — block
// keywords become block references, everything else is a schema type.
func IdentType(depth int, name string, symtab *SymbolTable) Type {
	typ, found := identType(depth, name, symtab)
	if !found {
		// TODO: Now sure the way we handle any type.
		return anyType(symtab)
	}
	return typ
}

func identType(depth int, name string, symtab *SymbolTable) (Type, bool) {
	if symtab != nil {
		if blockRef, ok := symtab.Lookup(name); ok {
			if depth <= 0 {
				return NewBlockRefType(name, blockRef.Block), true
			}
			if typ, found := identType(depth-1, blockRef.Block.Ast.Kind, symtab); found {
				return typ, true
			}

			// The Type for the blockRef.Block.Ast is not found but we have a block defined
			// so if it doesnt have a schema defined, what ever it is is it's own schema definition.
			//
			// i.e.
			//   let vars {
			//      key = "value"
			//      nums = [1, 2, 3]
			//   }
			// Here vars's kind is let and there is no `schema let {}` because let is an arbitary
			// kind the user has choosen, and it could be `foo bar {}` as well. so we construct
			// the block schema from the Ast.

			// TODO: we may not need the `inlineCounter` here because the name is already make it unique
			// and if two blocks have the same name it'll be an error, (the name is the global namespace).
			newSchemaName := "__anon_schema_of_" + name + "_" + fmt.Sprintf("%d", inlineCounter.Add(1))
			blockSchema := NewBlockSchema(
				blockRef.Block.Annotations,
				newSchemaName,
				blockRef.Block.Ast,
				symtab)
			return NewBlockRefType(newSchemaName, &blockSchema), true
		}
	}
	return Type{}, false
}

// subscriptionType returns the type of a subscription expression.
// In bootstrap mode (symbols == nil), first tries to resolve parameterized
// types like list[tool] or map[str], then falls back to subscript result
// type inference. With symbols, always infers from the object type.
func subscriptionType(depth int, e *ast.Subscription, symtab *SymbolTable) Type {
	// Bootstrap: try parameterized type like list[tool] or map[str].
	if baseIdent, ok := e.Object.(*ast.Identifier); ok {
		elemType := schemaFromExprWithDepth(depth, e.Index, symtab)
		switch baseIdent.Value {
		case "list":
			return NewListType(elemType)
		case "map":
			// NOTE: Map keys are always "str" but we set anyways maybe
			// in the future we support other key types.
			return NewMapType(IdentType(0, "str", symtab), elemType)
		}
	}
	return subscriptResultType(depth, schemaFromExprWithDepth(depth, e.Object, symtab), symtab)
}

// memberAccessType resolves the type of a member access expression
// (e.g. gpt4.model_name). Looks up the object's type, then finds
// the member's type in the corresponding block schema.
func memberAccessType(depth int, ma *ast.MemberAccess, symtab *SymbolTable) Type {
	// Incomplete member access (e.g. "gpt4." while typing).
	if ma.Member == "" {
		return anyType(symtab)
	}

	objType := schemaFromExprWithDepth(depth, ma.Object, symtab)

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
func subscriptResultType(depth int, t Type, symtab *SymbolTable) Type {
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
				return subscriptResultType(depth, m, symtab)
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
func mapLiteralType(depth int, m *ast.MapLiteral, symtab *SymbolTable) Type {
	if len(m.Entries) == 0 {
		return Type{Kind: Map}
	}

	// TODO: depth is not used here but it should be.

	// TODO: Check if the first element's type is compatible with
	// all the other elements' types. in that case, that can be the
	// map value type.
	//
	// first := ExprType(m.Entries[0].Value, symbols)
	// for _, entry := range m.Entries[1:] {
	// }

	return NewMapType(IdentType(0, "str", symtab), anyType(symtab))
}

// listLiteralType infers the type of a list literal. If all elements
// have the same type, returns list[T]. Otherwise returns an untyped list.
func listLiteralType(depth int, list *ast.ListLiteral, symbols *SymbolTable) Type {
	if len(list.Elements) == 0 {
		return Type{Kind: List}
	}

	// TODO: depth, symbols is not used here but it should be.

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
	if symtab != nil {
		return IdentType(0, "any", symtab)
	}
	return NewBlockRefType("any", nil)
}

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

func TypeOf(expr ast.Expression, symbols *SymbolTable) Type {
	// TODO: Cache the expression here so calling this on the same type multiple
	// times wont rerun the bellow function.
	return schemaFromExprWithDepth(1, expr, symbols)
}

// EvalType resolves the type of a value expression at depth 0.
// Unlike SchemaTypeFromExpr (depth 1), this returns the direct type of the
// expression without walking up the schema chain. Use this for lambda params
// and other value-level identifiers.
func EvalType(expr ast.Expression, symbols *SymbolTable) Type {
	return schemaFromExprWithDepth(0, expr, symbols)
}

// ExprToBlockBody returns the block body of the expression. Iff the resolved expression points
// to a block definitino or a block expression.
func ExprToBlockBody(expr ast.Expression, symbols *SymbolTable) *ast.BlockBody {
	return exprToBlockBody(expr, symbols)
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
	// in the bootstrap.orca file. as `bool true {}` and `bool false {}`
	//
	// Ideally we want the string, number to be the same like
	// `string "foo"` and `number 42` to be the same (consecptually), so
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
	// FIXME: Move the "number", "null" to constants.go file.
	case *ast.StringLiteral:
		return IdentType(depth-1, "string", symbols)
	case *ast.NumberLiteral:
		return IdentType(depth-1, "number", symbols)

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
	case *ast.TernaryExpression:
		return ternaryExprType(depth, e, symbols)
	case *ast.Lambda:
		return lambdaExprType(depth, e, symbols)
	case *ast.BlockExpression:
		return blockExprType(depth, e, symbols)
	default:
		return anyType(symbols)
	}
}

// ternaryExprType returns the type of a ternary expression.
// If both branches have the same type, returns that type.
// Otherwise returns a union of the two branch types, flattened.
func ternaryExprType(depth int, e *ast.TernaryExpression, symbols *SymbolTable) Type {
	trueType := schemaFromExprWithDepth(depth, e.TrueExpr, symbols)
	falseType := schemaFromExprWithDepth(depth, e.FalseExpr, symbols)

	if IsCompatible(trueType, falseType) && IsCompatible(falseType, trueType) {
		return trueType
	}

	// Collect members, flattening nested unions.
	var members []Type
	if trueType.Kind == Union {
		members = append(members, trueType.Members...)
	} else {
		members = append(members, trueType)
	}
	if falseType.Kind == Union {
		members = append(members, falseType.Members...)
	} else {
		members = append(members, falseType)
	}

	return NewUnionType(members...)
}

// blockExprType returns the type of an inline block expression.
// For schema blocks, registers the inline schema under a synthetic name
// so member access can resolve through it.
func blockExprType(depth int, e *ast.BlockExpression, symtab *SymbolTable) Type {

	if e.Name == "" {
		// TODO: Use the OrcaPrefix here but that mean we need to move the constant
		// out of the codegen to a common package.
		e.Name = fmt.Sprintf("_orca__anon_%d", symtab.nextInlineAnonID())
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
	//	 schema string {}
	//	 string "bar" {}
	//	 ExprType("bar") -> `schema string {}` and not `string "bar" {}`
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
	refBlock := NewBlockSchema(nil, e.Name, &e.BlockBody, symtab)

	// if expr = tool { ... }
	// refBlock = tool __anon_n {}
	// kindSchema = schema tool {}
	kindSchema := IdentType(0, e.BlockBody.Kind, symtab)
	if kindSchema.Kind == BlockRef {
		refBlock.Schema = kindSchema.Block
	}

	// Eagerly resolve the schema pointer for the returned kind type. The
	// kind name is a bootstrap schema (tool, agent, branch, ...) which is
	// in the symbol table by the time any inline expression is type-checked.
	// Resolving here means callers always get a fully-resolved Type — no
	// per-callsite "look up by kind name if Block is nil" workaround.
	result := NewBlockRefType(e.BlockBody.Kind, &refBlock)
	if schemaType, ok := symtab.Lookup(e.BlockBody.Kind); ok {
		result.Block = schemaType.Block
	}
	return result
}

// binaryExprType resolves the type of a binary expression. Pipe operators
// produce union types (e.g. string | null). Arithmetic operators (+, -, *, /)
// apply numeric promotion rules and string concatenation. Other operators
// return any.
func binaryExprType(depth int, e *ast.BinaryExpression, symbols *SymbolTable) Type {
	switch e.Operator.Type {
	case token.PIPE:
		// When both operands are schema types, | constructs a union type (e.g. string | null).
		// TODO: when both operands are numeric (int | int, int | float, etc.), | should be
		// treated as bitwise OR — this is not yet implemented.
		members := flattenUnionTypes(depth, e, symbols)
		if len(members) == 0 {
			return anyType(symbols)
		}
		return NewUnionType(members...)
	case token.PLUS, token.MINUS, token.STAR, token.SLASH:
		return arithmeticResultType(depth, e, symbols)
	case token.ARROW:
		leftType := schemaFromExprWithDepth(depth, e.Left, symbols)
		rightType := schemaFromExprWithDepth(depth, e.Right, symbols)
		if IsAnnotated(leftType, AnnotationWorkflowNode) && IsAnnotated(rightType, AnnotationWorkflowNode) {
			return IdentType(0, AnnotationWorkflowChain, symbols)
		}
	}
	return anyType(symbols)
}

// arithmeticResultType infers the result type of an arithmetic binary expression.
// Rules:
//   - string + string → string  (string concatenation, PLUS only)
//   - int op int → int
//   - float op float → float
//   - int op float / float op int → float  (numeric widening)
//   - otherwise → any
func arithmeticResultType(depth int, e *ast.BinaryExpression, symbols *SymbolTable) Type {
	left := schemaFromExprWithDepth(depth, e.Left, symbols)
	right := schemaFromExprWithDepth(depth, e.Right, symbols)

	// String concatenation: string + string → string.
	isLeftStr := IsCompatible(left, IdentType(0, BlockKindString, symbols))
	isRightStr := IsCompatible(right, IdentType(0, BlockKindString, symbols))
	if e.Operator.Type == token.PLUS && isLeftStr && isRightStr {
		return IdentType(0, BlockKindString, symbols)
	}

	// <number> op <number> → <number>.
	isLeftNum := IsCompatible(left, IdentType(0, BlockKindNumber, symbols))
	isRightNum := IsCompatible(right, IdentType(0, BlockKindNumber, symbols))
	if isLeftNum && isRightNum {
		return IdentType(0, BlockKindNumber, symbols)
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

			// TODO: we may not need the counter here because the name is already made unique
			// and if two blocks have the same name it'll be an error, (the name is the global namespace).
			newSchemaName := "_orca__anon_schema_of_" + name + "_" + fmt.Sprintf("%d", symtab.nextInlineAnonID())
			blockSchema := NewBlockSchema(
				blockRef.Block.Annotations,
				newSchemaName,
				blockRef.Block.Ast,
				symtab)
			return NewBlockRefType(newSchemaName, &blockSchema), true
		}
	}

	// This (negative depth) is only for testability.
	// Without a symbol table, schema type expressions at depth ≤ 0 treat identifiers as
	// named type references (user schema names) for codegen, bootstrap tests.
	if symtab == nil && depth <= 0 {
		return NewBlockRefType(name, nil), true
	}

	return Type{}, false
}

// subscriptionType returns the type of a subscription expression.
// In bootstrap mode (symbols == nil), first tries to resolve parameterized
// types like list[tool] or map[string], then falls back to subscript result
// type inference. With symbols, always infers from the object type.
func subscriptionType(depth int, e *ast.Subscription, symtab *SymbolTable) Type {
	// Bootstrap: try parameterized type like list[tool] or map[string].
	if baseIdent, ok := e.Object.(*ast.Identifier); ok && len(e.Indices) > 0 {
		switch baseIdent.Value {
		case "list":
			elemType := schemaFromExprWithDepth(depth, e.Indices[0], symtab)
			return NewListType(elemType)
		case "map":
			if len(e.Indices) == 2 {
				keyType := schemaFromExprWithDepth(depth, e.Indices[0], symtab)
				valType := schemaFromExprWithDepth(depth, e.Indices[1], symtab)
				return NewMapType(keyType, valType)
			}
			// map requires exactly 2 indices: map[key_type, value_type].
			return anyType(symtab)
		case "callable":
			return callableTypeFromIndices(depth, e.Indices, symtab)
		}
	}
	return subscriptResultType(depth, schemaFromExprWithDepth(depth, e.Object, symtab), symtab)
}

// callableTypeFromIndices builds a Callable type from subscription indices.
// The last index is the return type; all preceding indices are parameter types.
// callable[number, string, bool] → params=[number, string], return=bool.
func callableTypeFromIndices(depth int, indices []ast.Expression, symtab *SymbolTable) Type {
	if len(indices) == 0 {
		return anyType(symtab)
	}
	paramTypes := make([]Type, 0, len(indices)-1)
	for _, idx := range indices[:len(indices)-1] {
		paramTypes = append(paramTypes, schemaFromExprWithDepth(depth, idx, symtab))
	}
	returnType := schemaFromExprWithDepth(depth, indices[len(indices)-1], symtab)
	return NewCallableType(paramTypes, returnType)
}

// lambdaExprType returns the Callable type of a lambda expression.
// If a return type annotation is present, use it; otherwise infer from the body.
func lambdaExprType(depth int, e *ast.Lambda, symtab *SymbolTable) Type {
	paramTypes := make([]Type, len(e.Params))
	for i, p := range e.Params {
		paramTypes[i] = schemaFromExprWithDepth(depth, p.TypeExpr, symtab)
	}
	var retType Type
	if e.ReturnType != nil {
		retType = schemaFromExprWithDepth(depth, e.ReturnType, symtab)
	} else {
		// Infer return type from body. To resolve param references in the body,
		// we'd need a child symbol table, but for now return any.
		retType = anyType(symtab)
	}
	return NewCallableType(paramTypes, retType)
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

	return NewMapType(IdentType(0, "string", symtab), anyType(symtab))
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
		return IdentType(0, BlockKindAny, symtab)
	}
	return NewBlockRefType(BlockKindAny, nil)
}

func exprToBlockBody(expr ast.Expression, symbols *SymbolTable) *ast.BlockBody {
	switch e := expr.(type) {

	// TODO: Maybe consider string "foo" {}
	case *ast.StringLiteral:
	case *ast.NumberLiteral:
	case *ast.ListLiteral:
	case *ast.MapLiteral:
		return nil

	case *ast.Identifier:
		if symbols != nil {
			if typ, ok := symbols.Lookup(e.Value); ok && typ.Block != nil && typ.Block.Ast != nil {
				return typ.Block.Ast
			}
		}

	case *ast.MemberAccess:
		if rightBlock := exprToBlockBody(e.Object, symbols); rightBlock != nil {
			if assign := FindAssignment(rightBlock, e.Member); assign != nil {
				return exprToBlockBody(assign.Value, symbols)
			}
		}
		return nil
	// TODO: resolve return type from the callee's type.
	case *ast.Subscription:
		return nil
	case *ast.CallExpression:
		return nil
	case *ast.BinaryExpression:
		if symbols != nil && e.Operator.Type == token.ARROW {
			return &ast.BlockBody{
				// FIXME: OrcaPrefix should be moved out from langgraph package
				Name: fmt.Sprintf("%sinline_chain_%d", "_orca__", symbols.nextInlineAnonID()),
				Kind: AnnotationWorkflowChain,
				Assignments: []*ast.Assignment{
					{Name: "left", Value: e.Left},
					{Name: "right", Value: e.Right},
				},
			}
		}
		return nil
	case *ast.TernaryExpression:
		return nil
	case *ast.Lambda:
		return nil
	case *ast.BlockExpression:
		return &e.BlockBody
	}

	return nil
}

// FIXME: This is not the best place and this might be a duplicate function of some ast helper.
func FindAssignment(blockBody *ast.BlockBody, member string) *ast.Assignment {
	for _, assign := range blockBody.Assignments {
		if assign.Name == member {
			return assign
		}
	}
	return nil
}

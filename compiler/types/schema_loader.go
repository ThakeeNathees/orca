package types

import (
	_ "embed"
	"fmt"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
	"github.com/thakee/orca/compiler/token"
)

//go:embed builtins.oc
var schemaSource string

// loadSchemas parses the embedded builtins.oc file and builds
// a map of block type names to their field schemas. This is the
// single source of truth for block field definitions.
func loadSchemas() (map[string]BlockSchema, error) {
	l := lexer.New(schemaSource)
	p := parser.New(l)
	program := p.ParseProgram()

	if errs := p.Errors(); len(errs) > 0 {
		return nil, fmt.Errorf("failed to parse builtins.oc: %v", errs)
	}

	schemas := make(map[string]BlockSchema)

	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		// Only process schema blocks.
		if block.TokenStart.Type != token.SCHEMA {
			continue
		}

		schema, err := SchemaFromBlock(block)
		if err != nil {
			return nil, err
		}
		schemas[block.Name] = schema
	}

	return schemas, nil
}

// SchemaFromBlock builds a BlockSchema from a schema block's assignments.
// Each assignment is resolved using ResolveFieldSchema, producing field
// types, required flags, and descriptions from annotations.
func SchemaFromBlock(block *ast.BlockStatement) (BlockSchema, error) {
	fields := make(map[string]FieldSchema)
	for _, assign := range block.Assignments {
		fs, err := ResolveFieldSchema(assign)
		if err != nil {
			return BlockSchema{}, fmt.Errorf("schema %s.%s: %w", block.Name, assign.Name, err)
		}
		fields[assign.Name] = fs
	}
	return BlockSchema{Fields: fields}, nil
}

// ResolveFieldSchema extracts a FieldSchema from an assignment using the
// annotation-based format. The assignment value is the type expression
// (e.g. str, str | model | null). Annotations provide metadata:
// @desc("...") for descriptions. Required is inferred: if the type
// contains null in a union, the field is optional; otherwise required.
func ResolveFieldSchema(assign *ast.Assignment) (FieldSchema, error) {
	typ, err := resolveType(assign.Value)
	if err != nil {
		return FieldSchema{}, fmt.Errorf("type: %w", err)
	}

	var fs FieldSchema

	// If the type is a union containing null, the field is optional.
	// Strip null from the stored type — it's just an optionality marker.
	if typ.Kind == Union {
		var nonNull []Type
		for _, m := range typ.Members {
			if m.IsNull() {
				continue
			}
			nonNull = append(nonNull, m)
		}
		if len(nonNull) < len(typ.Members) {
			// Had null member — field is optional.
			fs.Required = false
			if len(nonNull) == 1 {
				fs.Type = nonNull[0]
			} else {
				fs.Type = NewUnionType(nonNull...)
			}
		} else {
			fs.Required = true
			fs.Type = typ
		}
	} else {
		fs.Required = true
		fs.Type = typ
	}

	// Extract metadata from annotations.
	for _, ann := range assign.Annotations {
		switch ann.Name {
		case "desc":
			if len(ann.Arguments) == 1 {
				if strLit, ok := ann.Arguments[0].(*ast.StringLiteral); ok {
					fs.Description = strLit.Value
				}
			}
		}
	}

	return fs, nil
}

// resolveType converts a type expression from the .oc file into an
// internal Type. Handles identifiers (str, int, model, etc.), and
// binary expressions with pipe for union types (str | model).
func resolveType(expr ast.Expression) (Type, error) {
	switch e := expr.(type) {
	case *ast.NullLiteral:
		return TypeOf("null"), nil

	case *ast.Identifier:
		return resolveIdentType(e.Value)

	case *ast.Subscription:
		// Parameterized type like list[tool] or map[str].
		baseIdent, ok := e.Object.(*ast.Identifier)
		if !ok {
			return Type{}, fmt.Errorf("expected identifier for parameterized type, got %T", e.Object)
		}
		elemType, err := resolveType(e.Index)
		if err != nil {
			return Type{}, fmt.Errorf("%s[...]: %w", baseIdent.Value, err)
		}
		switch baseIdent.Value {
		case "list":
			return NewListType(elemType), nil
		case "map":
			return NewMapType(TypeOf("str"), elemType), nil
		default:
			return Type{}, fmt.Errorf("parameterized type not supported for %q", baseIdent.Value)
		}

	case *ast.BinaryExpression:
		if e.Operator.Type != token.PIPE {
			return Type{}, fmt.Errorf("unexpected operator %q in type expression", e.Operator.Literal)
		}
		// Collect all union members by flattening nested pipes.
		members, err := flattenUnion(expr)
		if err != nil {
			return Type{}, err
		}
		return NewUnionType(members...), nil

	default:
		return Type{}, fmt.Errorf("unexpected expression %T in type position", expr)
	}
}

// flattenUnion recursively collects all members of a pipe-separated
// union expression into a flat slice. Handles both `a | b` and
// chained `a | b | c`.
func flattenUnion(expr ast.Expression) ([]Type, error) {
	switch e := expr.(type) {
	case *ast.NullLiteral:
		return []Type{TypeOf("null")}, nil

	case *ast.Identifier:
		typ, err := resolveIdentType(e.Value)
		if err != nil {
			return nil, err
		}
		return []Type{typ}, nil

	case *ast.Subscription:
		typ, err := resolveType(expr)
		if err != nil {
			return nil, err
		}
		return []Type{typ}, nil

	case *ast.BinaryExpression:
		if e.Operator.Type != token.PIPE {
			return nil, fmt.Errorf("unexpected operator %q in union", e.Operator.Literal)
		}
		left, err := flattenUnion(e.Left)
		if err != nil {
			return nil, err
		}
		right, err := flattenUnion(e.Right)
		if err != nil {
			return nil, err
		}
		return append(left, right...), nil

	default:
		return nil, fmt.Errorf("unexpected expression %T in union", expr)
	}
}

// resolveIdentType maps an identifier name to an internal Type.
// All names resolve to BlockRef with the name as BlockType. Block type
// names (model, agent, etc.) use their BlockKind constant; everything
// else (str, int, user schemas) uses the name directly.
func resolveIdentType(name string) (Type, error) {
	if kind, ok := BlockKindFromName(name); ok {
		return NewBlockRefType(kind), nil
	}
	// Treat as a named type (primitive or user-defined schema).
	return Type{Kind: BlockRef, BlockType: BlockKind(name)}, nil
}

// init loads the embedded schemas and replaces the blockSchemas map.
func init() {
	schemas, err := loadSchemas()
	if err != nil {
		panic(fmt.Sprintf("failed to load embedded schemas: %v", err))
	}
	blockSchemas = schemas
	builtinNames = make([]string, 0, len(schemas))
	typeCache = make(map[BlockKind]Type, len(schemas))
	for name := range schemas {
		builtinNames = append(builtinNames, name)
		typeCache[BlockKind(name)] = Type{Kind: BlockRef, BlockType: BlockKind(name)}
	}
}

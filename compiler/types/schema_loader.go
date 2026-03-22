package types

import (
	_ "embed"
	"fmt"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
	"github.com/thakee/orca/compiler/token"
)

//go:embed block_schemas.oc
var schemaSource string

// loadSchemas parses the embedded block_schemas.oc file and builds
// a map of block type names to their field schemas. This is the
// single source of truth for block field definitions.
func loadSchemas() (map[string]BlockSchema, error) {
	l := lexer.New(schemaSource)
	p := parser.New(l)
	program := p.ParseProgram()

	if errs := p.Errors(); len(errs) > 0 {
		return nil, fmt.Errorf("failed to parse block_schemas.oc: %v", errs)
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

		fields := make(map[string]FieldSchema)
		for _, assign := range block.Assignments {
			fieldSchema, err := resolveFieldSchema(assign.Value)
			if err != nil {
				return nil, fmt.Errorf("schema %s.%s: %w", block.Name, assign.Name, err)
			}
			fields[assign.Name] = fieldSchema
		}

		schemas[block.Name] = BlockSchema{Fields: fields}
	}

	return schemas, nil
}

// resolveFieldSchema extracts a FieldSchema from a map literal expression
// like {type: str, required: true}.
func resolveFieldSchema(expr ast.Expression) (FieldSchema, error) {
	mapLit, ok := expr.(*ast.MapLiteral)
	if !ok {
		return FieldSchema{}, fmt.Errorf("expected map literal, got %T", expr)
	}

	var fs FieldSchema
	var hasType bool

	for _, entry := range mapLit.Entries {
		key, ok := entry.Key.(*ast.Identifier)
		if !ok {
			return FieldSchema{}, fmt.Errorf("expected identifier key, got %T", entry.Key)
		}

		switch key.Value {
		case "type":
			typ, err := resolveType(entry.Value)
			if err != nil {
				return FieldSchema{}, fmt.Errorf("type: %w", err)
			}
			fs.Type = typ
			hasType = true
		case "required":
			boolLit, ok := entry.Value.(*ast.BooleanLiteral)
			if !ok {
				return FieldSchema{}, fmt.Errorf("required: expected boolean, got %T", entry.Value)
			}
			fs.Required = boolLit.Value
		case "desc":
			strLit, ok := entry.Value.(*ast.StringLiteral)
			if !ok {
				return FieldSchema{}, fmt.Errorf("desc: expected string, got %T", entry.Value)
			}
			fs.Description = strLit.Value
		default:
			return FieldSchema{}, fmt.Errorf("unknown field property %q", key.Value)
		}
	}

	if !hasType {
		return FieldSchema{}, fmt.Errorf("missing 'type' property")
	}

	return fs, nil
}

// resolveType converts a type expression from the .oc file into an
// internal Type. Handles identifiers (str, int, model, etc.), and
// binary expressions with pipe for union types (str | model).
func resolveType(expr ast.Expression) (Type, error) {
	switch e := expr.(type) {
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
			return NewMapType(StringType, elemType), nil
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
	case *ast.Identifier:
		typ, err := resolveIdentType(e.Value)
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

// resolveIdentType maps a type annotation identifier to an internal Type.
// Type annotation keywords (str, int, float, bool, list, map, any) map
// to their corresponding primitive types. Other identifiers are treated
// as block references (e.g., "model" → BlockRef("model")).
func resolveIdentType(name string) (Type, error) {
	// Check if it's a type annotation keyword.
	if typ, ok := TypeFromAnnotation(name); ok {
		return typ, nil
	}

	// Otherwise treat it as a block reference.
	kind, ok := BlockKindFromName(name)
	if !ok {
		return Type{}, fmt.Errorf("unknown type %q", name)
	}
	return NewBlockRefType(kind), nil
}

// BlockKindFromName maps a block type name string to its BlockKind constant.
var blockKindMap = map[string]BlockKind{
	"model":     BlockModel,
	"agent":     BlockAgent,
	"tool":      BlockTool,
	"task":      BlockTask,
	"knowledge": BlockKnowledge,
	"workflow":  BlockWorkflow,
	"trigger":   BlockTrigger,
}

// BlockKindFromName returns the BlockKind for a given block type name.
func BlockKindFromName(name string) (BlockKind, bool) {
	kind, ok := blockKindMap[name]
	return kind, ok
}

// init loads the embedded schemas and replaces the blockSchemas map.
func init() {
	schemas, err := loadSchemas()
	if err != nil {
		panic(fmt.Sprintf("failed to load embedded schemas: %v", err))
	}
	blockSchemas = schemas
}

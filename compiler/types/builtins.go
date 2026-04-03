package types

import (
	_ "embed"
	"fmt"
	"sync/atomic"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
	"github.com/thakee/orca/compiler/token"
)

// inlineCounter generates unique names for anonymous inline schemas.
var inlineCounter atomic.Int64

//go:embed builtins.oc
var schemaSource string

// loadSchemas parses the embedded builtins.oc file and builds
// a map of block type names to their field schemas. This is the
// single source of truth for block field definitions.
func loadSchemas() (map[string]BlockSchema, error) {
	l := lexer.New(schemaSource, "")
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
	schema, err := SchemaFromAssignments(block.Assignments)
	if err != nil {
		return BlockSchema{}, fmt.Errorf("schema %s: %w", block.Name, err)
	}
	return schema, nil
}

// SchemaFromAssignments builds a BlockSchema from a slice of assignments.
// Used by both named schema blocks and inline schema expressions.
func SchemaFromAssignments(assignments []*ast.Assignment) (BlockSchema, error) {
	fields := make(map[string]FieldSchema)
	for _, assign := range assignments {
		fs := ResolveFieldSchema(assign)
		fields[assign.Name] = fs
	}
	return BlockSchema{Fields: fields}, nil
}

// ResolveFieldSchema extracts a FieldSchema from an assignment using the
// annotation-based format. The assignment value is the type expression
// (e.g. str, str | model | null). Annotations provide metadata:
// @desc("...") for descriptions. Required is inferred: if the type
// contains null in a union, the field is optional; otherwise required.
func ResolveFieldSchema(assign *ast.Assignment) FieldSchema {
	// Resolve the type using ExprType in bootstrap mode (nil symbol table).
	typ := ExprType(assign.Value, nil)

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

	return fs
}

// init loads the embedded schemas and replaces the blockSchemas map.
func init() {
	schemas, err := loadSchemas()
	if err != nil {
		panic(fmt.Sprintf("failed to load embedded schemas: %v", err))
	}
	blockSchemas = schemas
	builtinNames = make([]string, 0, len(schemas))
	for name := range schemas {
		builtinNames = append(builtinNames, name)
	}
}

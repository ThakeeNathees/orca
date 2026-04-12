package types

import (
	_ "embed"
	"fmt"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
)

//go:embed bootstrap.orca
var BootstrapSource string

type BootstrapResult struct {
	Schemas []BlockSchema
	Symtab  *SymbolTable
}

// Bootstrap parses the embedded bootstrap.orca file and returns
// a list of block schemas. This is the single source of truth
// for block domain specific definitions. Other than the
// bootstrap.orca file the langauge itself is domain agnostic.
func Bootstrap(bootstrapSource string) BootstrapResult {

	l := lexer.New(bootstrapSource, "")
	p := parser.New(l)
	program := p.ParseProgram()

	// TODO: Better way to panic
	if errs := p.Errors(); len(errs) > 0 {
		panic(fmt.Errorf("failed to parse bootstrap.orca: %v", errs))
	}

	symtab := NewSymbolTable()
	schemas := make([]BlockSchema, 0)

	// Since this is bootstrapped, the symbol table is updated over the iteration so
	// if we reference a symbol (e.g. string) before it defined it wont work. This is
	// the only exception where the order of the statements matters.
	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		schema := NewBlockSchema(block.Annotations, block.Name, &block.BlockBody, &symtab)
		symtab.Define(block.Name, NewBlockRefType(block.Name, &schema), block.NameToken)
		schemas = append(schemas, schema)
	}

	return BootstrapResult{
		Schemas: schemas,
		Symtab:  &symtab,
	}
}

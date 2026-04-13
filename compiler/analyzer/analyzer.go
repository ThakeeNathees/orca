// Package analyzer performs semantic analysis on the AST produced by the parser.
// It resolves references between blocks (e.g., verifying that an agent's model
// refers to a defined model block), checks for type mismatches, undefined
// identifiers, and other errors that can't be caught by syntax alone.
package analyzer

import (
	"fmt"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/types"
)

// AnalyzedProgram holds the output of semantic analysis: the symbol table
// built from block definitions and any diagnostics produced.
type AnalyzedProgram struct {
	Ast         *ast.Program
	SymbolTable *types.SymbolTable
	Diagnostics []diagnostic.Diagnostic

	// BlockOrder is the topologically sorted list of user-defined block names.
	// Blocks with no dependencies come first; dependents come after their
	// dependencies. Codegen uses this to emit blocks in valid definition order.
	BlockOrder []string
}

// Analyze walks the AST and performs semantic analysis.
// Builds a symbol table from all block definitions, then validates
// each block's fields against its schema. Returns the symbol table
// along with diagnostics so callers (like the LSP) can use it for
// hover, go-to-definition, and other features.
func Analyze(program *ast.Program) AnalyzedProgram {

	// Bootstrap the schema file.
	bootstrapResult := types.Bootstrap(types.BootstrapSource)

	ap := AnalyzedProgram{
		Ast:         program,
		SymbolTable: bootstrapResult.Symtab,
		Diagnostics: []diagnostic.Diagnostic{},
	}

	// These function should run in this order
	injectAnonBlocks(&ap)
	buildSymbolTable(&ap)
	resolveBlockSchemaReferences(&ap)
	buildBlockDependencyGraph(&ap)

	for _, stmt := range program.Statements {
		// We dont have any other statement than BlockStatement, maybe
		// we can just remove Statement and use BlockStatement directly.
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}

		// Analyze and get the non-suppressed diagnostics for the block.
		var blockDiags []diagnostic.Diagnostic
		blockDiags = analyzeBlock(block, ap.SymbolTable)
		codes, all := suppressedCodes(block.Annotations)
		blockDiags = filterSuppressed(blockDiags, codes, all)

		// Add the block diagnostics to the program diagnostics.
		ap.Diagnostics = append(ap.Diagnostics, blockDiags...)
	}

	return ap
}

// injectAnonBlocks injects the anonymous blocks into the symbol table.
func injectAnonBlocks(ap *AnalyzedProgram) {
	ast.Walk(ap.Ast, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.BlockExpression:
			if e != nil {
				// Resolve the type first to ensure BlockNameAnon is set and the
				// inline block is registered in the symbol table.
				// This TypeOf will set the symbol table as well which is ugly side effect
				// interms of functional programming and buggy, but works, maybe I need to think.
				types.TypeOf(e, ap.SymbolTable)
				bs := ast.BlockStatement{
					BlockBody: e.BlockBody,
					NameToken: e.Start(), // Actually they dont have a name token (cause anon)
				}
				bs.BlockBody = e.BlockBody
				ap.Ast.Statements = append(ap.Ast.Statements, &bs)
			}
		}
		return true
	})
}

// buildSymbolTable walks all block statements and registers each block
// name with its block reference type. Reports duplicate block names.
func buildSymbolTable(ap *AnalyzedProgram) {

	for _, stmt := range ap.Ast.Statements {

		// NOTE: Block is and will always be the only statement supported by Orca
		// Probably we dont need this generic statement base class.
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}

		// TODO: Split the bellow logic into smaller function for readability.
		if _, exists := ap.SymbolTable.Lookup(block.Name); exists {
			codes, all := suppressedCodes(block.Annotations)
			if !all && !codes[diagnostic.CodeDuplicateBlock] {
				ap.Diagnostics = append(ap.Diagnostics, diagnostic.Diagnostic{
					Severity:    diagnostic.Error,
					Code:        diagnostic.CodeDuplicateBlock,
					Position:    diagnostic.PositionOf(block.NameToken),
					EndPosition: diagnostic.EndPositionOf(block.NameToken),
					Message:     fmt.Sprintf("duplicate block name %q", block.Name),
					Source:      "analyzer",
					File:        block.SourceFile,
				})
			}
		}

		schema := types.NewBlockSchema(block.Annotations, block.Name, &block.BlockBody, ap.SymbolTable)
		typ := types.NewBlockRefType(block.Name, &schema)
		ap.SymbolTable.Define(block.Name, typ, block.NameToken)
	}
}

// FIXME: This might cause a stack overflow if the schema is recursive.
// Add a depth parameter to the function.
func resolveFieldSchemaReferences(bs *types.BlockSchema, st *types.SymbolTable) {
	for _, fieldSchema := range bs.Fields {
		resolveTypeReference(&fieldSchema.Type, st)
	}
}

// resolveTypeReference resolves the type reference to the actual block schema.
// Type struct has BlockRef that and they needs to be resolved to the actual block schema.
func resolveTypeReference(typ *types.Type, st *types.SymbolTable) {
	if typ.Kind != types.BlockRef {
		return
	}

	if typ.Block == nil {
		if ref, ok := st.Lookup(typ.BlockName); ok {
			typ.Block = ref.Block
		}
	}

	if typ.Block != nil {
		resolveFieldSchemaReferences(typ.Block, st)
	}

	if typ.Block != nil && typ.Block.Schema == nil {
		if schemaRef, ok := st.Lookup(typ.Block.Ast.Kind); ok {
			typ.Block.Schema = schemaRef.Block
		}
	}

	if typ.Block != nil && typ.Block.Schema != nil {
		resolveFieldSchemaReferences(typ.Block.Schema, st)
	}
}

func resolveBlockSchemaReferences(ap *AnalyzedProgram) {

	// Bootstrap the schema's schema.
	//
	//   +---------------------------------------+
	//   | schema's schema is the schema itself. |
	//   +---------------------------------------+
	//
	if schemaSchema, ok := ap.SymbolTable.Lookup(types.BlockKindSchema); ok {
		schemaSchema.Block.Schema = schemaSchema.Block
	}

	for _, symbol := range ap.SymbolTable.GetSymbols() {
		resolveTypeReference(&symbol.Type, ap.SymbolTable)
	}
}

// suppressedCodes extracts the set of diagnostic codes suppressed by
// @suppress annotations. @suppress with no args suppresses all codes.
// @suppress("code1", "code2") suppresses specific codes.
// Returns the set of suppressed codes, and whether all codes are suppressed.
func suppressedCodes(annotations []*ast.Annotation) (codes map[string]bool, all bool) {
	for _, ann := range annotations {
		if ann.Name != "suppress" {
			continue
		}
		if len(ann.Arguments) == 0 {
			return nil, true
		}
		if codes == nil {
			codes = make(map[string]bool)
		}
		for _, arg := range ann.Arguments {
			if strLit, ok := arg.(*ast.StringLiteral); ok {
				codes[strLit.Value] = true
			}
		}
	}
	return codes, false
}

// filterSuppressed removes diagnostics that are suppressed by the given
// annotation set. If suppressAll is true, all diagnostics are removed.
func filterSuppressed(diags []diagnostic.Diagnostic, codes map[string]bool, suppressAll bool) []diagnostic.Diagnostic {
	if suppressAll {
		return nil
	}
	if len(codes) == 0 {
		return diags
	}
	var filtered []diagnostic.Diagnostic
	for _, d := range diags {
		if !codes[d.Code] {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

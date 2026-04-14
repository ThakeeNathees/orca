package types

import (
	_ "embed"
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
	"github.com/thakee/orca/compiler/token"
)

//go:embed bootstrap.orca
var testBootstrapSource string

// typePtr returns a pointer to a Type for use in test table comparisons.
func typePtr(t Type) *Type { return &t }

// TestLoadSchemas verifies that the embedded builtins.orca is parsed and
// all expected schemas are present with the correct number of fields.
func TestLoadSchemas(t *testing.T) {
	res := Bootstrap(testBootstrapSource)
	schemas := make(map[string]BlockSchema)
	for _, s := range res.Schemas {
		schemas[s.BlockName] = s
	}

	tests := []struct {
		name      string
		blockType string
		numFields int
	}{
		{"schema", BlockKindSchema, 0},
		{"string", "string", 0},
		{"number", "number", 0},
		{"bool", "bool", 0},
		{"list", "list", 0},
		{"map", "map", 0},
		{"any", "any", 0},
		{"null", "null", 0},
		{"model", "model", 5},
		{"agent", "agent", 5},
		{"tool", "tool", 4},
		{"workflow", "workflow", 2},
		{"cron", "cron", 2},
		{"webhook", "webhook", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, ok := schemas[tt.blockType]
			if !ok {
				t.Fatalf("schema %q not found", tt.blockType)
			}
			if len(schema.Fields) != tt.numFields {
				t.Errorf("num fields = %d, want %d", len(schema.Fields), tt.numFields)
			}
		})
	}
}

// TestLoadSchemasFieldTypes verifies that field types are correctly
// resolved from the .orca file, including union types and block references.
func TestLoadSchemasFieldTypes(t *testing.T) {
	res := Bootstrap(testBootstrapSource)
	schemas := make(map[string]BlockSchema)
	for _, s := range res.Schemas {
		schemas[s.BlockName] = s
	}

	// wantString is the canonical Orca type string Type.String() should produce for each
	// field per bootstrap.orca (string, model, number, schema, … — not the meta-type name "schema"
	// unless the field is literally typed as `schema`).
	tests := []struct {
		name     string
		block    string
		field    string
		kind     TypeKind
		required bool
		wantStr  string
	}{
		{"agent.tools", "agent", "tools", Union, false, "list[tool] | nulltype"},
		{"model.model_name", "model", "model_name", BlockRef, true, "string"},
		{"agent.model", "agent", "model", Union, true, "string | model"},
		{"model.provider", "model", "provider", BlockRef, true, "string"},
		{"model.temperature", "model", "temperature", Union, false, "number | nulltype"},
		{"agent.persona", "agent", "persona", BlockRef, true, "string"},
		{"tool.desc", "tool", "desc", Union, false, "string | nulltype"},
		{"tool.invoke", "tool", "invoke", BlockRef, true, "callable"},
		{"workflow.name", "workflow", "name", Union, false, "string | nulltype"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := schemas[tt.block].Fields[tt.field]
			if !ok {
				t.Fatalf("field %s.%s not found", tt.block, tt.field)
			}
			if field.Type.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", field.Type.Kind, tt.kind)
			}
			if got := field.Type.String(); got != tt.wantStr {
				t.Errorf("Type.String() = %q, want %q", got, tt.wantStr)
			}
			if field.Required != tt.required {
				t.Errorf("Required = %v, want %v", field.Required, tt.required)
			}
		})
	}
}

// TestLoadSchemasDescriptions verifies that @desc annotations are
// correctly extracted into FieldSchema.Description.
func TestLoadSchemasDescriptions(t *testing.T) {
	res := Bootstrap(testBootstrapSource)
	schemas := make(map[string]BlockSchema)
	for _, s := range res.Schemas {
		schemas[s.BlockName] = s
	}

	tests := []struct {
		name     string
		block    string
		field    string
		wantDesc bool
	}{
		{"model.provider has desc", "model", "provider", true},
		{"agent.model has desc", "agent", "model", true},
		{"tool.invoke has desc", "tool", "invoke", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := schemas[tt.block].Fields[tt.field]
			if !ok {
				t.Fatalf("field %s.%s not found", tt.block, tt.field)
			}
			hasDesc := field.Description != ""
			if hasDesc != tt.wantDesc {
				t.Errorf("has description = %v, want %v (desc=%q)", hasDesc, tt.wantDesc, field.Description)
			}
		})
	}
}

// TestResolveIdentAsType verifies that ExprTypeFromExpr on an identifier
// with a bootstrapped symbol table resolves each name to the corresponding
// schema block (or `any` when the name is unknown).
func TestResolveIdentAsType(t *testing.T) {
	res := Bootstrap(testBootstrapSource)
	st := res.Symtab

	tests := []struct {
		name  string
		input string
	}{
		{"primitive string", "string"},
		{"primitive number", "number"},
		{"primitive bool", "bool"},
		{"primitive any", "any"},
		{"block type model", "model"},
		{"block type agent", "agent"},
		{"block type cron", "cron"},
		{"block type webhook", "webhook"},
		{"block type schema", "schema"},
		{"unknown name falls back to any", "vpc_data_t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := EvalType(&ast.Identifier{Value: tt.input}, st)
			var want Type
			if tt.input == "vpc_data_t" {
				want, _ = st.Lookup("any")
			} else {
				want, _ = st.Lookup(tt.input)
			}
			if typ.Kind != BlockRef {
				t.Errorf("Kind = %v, want BlockRef", typ.Kind)
			}
			if !typ.Equals(want) {
				t.Errorf("ExprTypeFromExpr(ident %q) = %s, want %s", tt.input, typ.String(), want.String())
			}
		})
	}
}

// TestExprTypeFromExprBootstrap verifies ExprTypeFromExpr with a
// bootstrapped symbol table handles type expression forms correctly.
func TestExprTypeFromExprBootstrap(t *testing.T) {
	st := bootstrapSymtab(t)

	anyTyp := IdentType(0, "any", st)
	tests := []struct {
		name       string
		expr       ast.Expression
		kind       TypeKind
		expType    *Type  // expected via Equals when set (nil to skip)
		wantLookup string // if set, expected type is st.Lookup(wantLookup) (same Block pointer as ExprType)
	}{
		{
			"identifier string resolves to string",
			&ast.Identifier{Value: "string"},
			BlockRef, nil, "string",
		},
		{
			"identifier model resolves to model ref",
			&ast.Identifier{Value: "model"},
			BlockRef, nil, "model",
		},
		{
			"subscription list[string] resolves to list type",
			&ast.Subscription{
				Object:  &ast.Identifier{Value: "list"},
				Indices: []ast.Expression{&ast.Identifier{Value: "string"}},
			},
			List, nil, "",
		},
		{
			"subscription map[string, number] resolves to map type",
			&ast.Subscription{
				Object:  &ast.Identifier{Value: "map"},
				Indices: []ast.Expression{&ast.Identifier{Value: "string"}, &ast.Identifier{Value: "number"}},
			},
			Map, nil, "",
		},
		{
			"unsupported parameterized type returns any",
			&ast.Subscription{
				Object:  &ast.Identifier{Value: "set"},
				Indices: []ast.Expression{&ast.Identifier{Value: "string"}},
			},
			BlockRef, typePtr(anyTyp), "",
		},
		{
			"union via binary pipe expression",
			&ast.BinaryExpression{
				Left:     &ast.Identifier{Value: "string"},
				Operator: token.Token{Type: token.PIPE},
				Right:    &ast.Identifier{Value: "number"},
			},
			Union, nil, "",
		},
		{
			"binary expression with non-pipe operator returns any",
			&ast.BinaryExpression{
				Left:     &ast.Identifier{Value: "string"},
				Operator: token.Token{Type: token.PLUS, Literal: "+"},
				Right:    &ast.Identifier{Value: "number"},
			},
			BlockRef, typePtr(anyTyp), "",
		},
		{
			"number literal returns number",
			&ast.NumberLiteral{Value: 42},
			BlockRef, typePtr(IdentType(0, "number", st)), "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := EvalType(tt.expr, st)
			if typ.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", typ.Kind, tt.kind)
			}
			switch {
			case tt.wantLookup != "":
				want, ok := st.Lookup(tt.wantLookup)
				if !ok {
					t.Fatalf("lookup %q not found", tt.wantLookup)
				}
				if !typ.Equals(want) {
					t.Errorf("Type = %s, want %s", typ.String(), want.String())
				}
			case tt.expType != nil && !typ.Equals(*tt.expType):
				t.Errorf("Type = %s, want %s", typ.String(), tt.expType.String())
			}
		})
	}
}

// TestExprTypeFromExprInlineSchema verifies that inline schema expressions
// register a synthetic name in the symbol table and that the returned type
// has the schema kind with the schema pointer eagerly resolved from the
// symbol table. Callers no longer need a per-call "fallback if Block is nil"
// workaround because blockExprType resolves the kind name on return.
func TestExprTypeFromExprInlineSchema(t *testing.T) {
	st := bootstrapSymtab(t)
	expr := &ast.BlockExpression{
		BlockBody: ast.BlockBody{
			Kind: BlockKindSchema,
			Assignments: []*ast.Assignment{
				{
					Name:  "host",
					Value: &ast.Identifier{Value: "string"},
				},
			},
		},
	}

	typ := EvalType(expr, st)
	if typ.Kind != BlockRef {
		t.Errorf("Kind = %v, want BlockRef", typ.Kind)
	}
	if typ.BlockName != BlockKindSchema {
		t.Errorf("BlockName = %q, want %q", typ.BlockName, BlockKindSchema)
	}
	if typ.Block == nil {
		t.Errorf("Block = nil, want resolved schema pointer (eagerly looked up by kind)")
	} else if typ.Block.BlockName != BlockKindSchema {
		t.Errorf("Block.BlockName = %q, want %q", typ.Block.BlockName, BlockKindSchema)
	}
}

// TestFieldSchemaOptionalNullInUnion verifies NewBlockSchema marks a field optional
// when the type is a union that contains null (via Type.IsNull()), but keeps the full
// union on FieldSchema.Type — null is only an optionality marker, not removed from the type.
func TestFieldSchemaOptionalNullInUnion(t *testing.T) {
	st := Bootstrap(testBootstrapSource).Symtab
	tests := []struct {
		name        string
		input       string
		required    bool
		resultKind  TypeKind
		wantStr     string
		memberCount int
	}{
		{
			"string | nulltype optional, union retains null",
			"schema test_strip {\n  @suppress(\"duplicate-block\")\n  field = string | nulltype\n}",
			false, Union, "string | nulltype", 2,
		},
		{
			"string | model | nulltype optional, full union",
			"schema test_strip2 {\n  @suppress(\"duplicate-block\")\n  field = string | model | nulltype\n}",
			false, Union, "string | model | nulltype", 3,
		},
		{
			"string (no union) is required BlockRef",
			"schema test_strip3 {\n  @suppress(\"duplicate-block\")\n  field = string\n}",
			true, BlockRef, "string", 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input, "")
			p := parser.New(l)
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				t.Fatalf("parse errors: %v", p.Errors())
			}
			block := program.Statements[0].(*ast.BlockStatement)
			schema := NewBlockSchema(block.Annotations, block.Name, &block.BlockBody, st)
			fs := schema.Fields["field"]
			if fs.Required != tt.required {
				t.Errorf("Required = %v, want %v", fs.Required, tt.required)
			}
			if fs.Type.Kind != tt.resultKind {
				t.Errorf("Kind = %v, want %v", fs.Type.Kind, tt.resultKind)
			}
			if got := fs.Type.String(); got != tt.wantStr {
				t.Errorf("Type.String() = %q, want %q", got, tt.wantStr)
			}
			if tt.memberCount > 0 && len(fs.Type.Members) != tt.memberCount {
				t.Errorf("Members = %d, want %d", len(fs.Type.Members), tt.memberCount)
			}
		})
	}
}

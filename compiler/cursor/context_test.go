package cursor

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
)

// parseProgram is a test helper that parses input and fails on parse errors.
func parseProgram(t *testing.T, input string) *ast.Program {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	return program
}

// TestResolveTopLevel verifies that positions outside any block return TopLevel.
func TestResolveTopLevel(t *testing.T) {
	input := `model gpt4 { provider = "openai" }`
	program := parseProgram(t, input)

	tests := []struct {
		name   string
		line   int
		col    int
		expect CursorPosition
	}{
		{"before block", 1, 1, TopLevel},
		{"after block on same line", 1, 35, TopLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := Resolve(program, tt.line, tt.col)
			if ctx.Position != tt.expect {
				t.Errorf("Position = %v, want %v", ctx.Position, tt.expect)
			}
			if ctx.Block != nil {
				t.Errorf("Block should be nil at top level")
			}
		})
	}
}

// TestResolveBlockBody verifies that positions inside a block body
// (where a field name is expected) return BlockBody.
func TestResolveBlockBody(t *testing.T) {
	input := "model gpt4 {\n  provider = \"openai\"\n}"
	program := parseProgram(t, input)

	tests := []struct {
		name      string
		line      int
		col       int
		expect    CursorPosition
		blockType string
		blockName string
	}{
		{"start of field line", 2, 1, BlockBody, "model", "gpt4"},
		{"on closing brace line", 3, 1, BlockBody, "model", "gpt4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := Resolve(program, tt.line, tt.col)
			if ctx.Position != tt.expect {
				t.Errorf("Position = %v, want %v", ctx.Position, tt.expect)
			}
			if ctx.Block == nil {
				t.Fatalf("Block should not be nil")
			}
			if ctx.BlockType != tt.blockType {
				t.Errorf("BlockType = %q, want %q", ctx.BlockType, tt.blockType)
			}
			if ctx.Block.Name != tt.blockName {
				t.Errorf("Block.Name = %q, want %q", ctx.Block.Name, tt.blockName)
			}
		})
	}
}

// TestResolveFieldValue verifies that positions on a value expression
// return FieldValue with the enclosing assignment.
func TestResolveFieldValue(t *testing.T) {
	input := "model gpt4 {\n  provider = \"openai\"\n}"
	program := parseProgram(t, input)

	ctx := Resolve(program, 2, 14)
	if ctx.Position != FieldValue {
		t.Errorf("Position = %v, want FieldValue", ctx.Position)
	}
	if ctx.Assignment == nil {
		t.Fatalf("Assignment should not be nil")
	}
	if ctx.Assignment.Name != "provider" {
		t.Errorf("Assignment.Name = %q, want %q", ctx.Assignment.Name, "provider")
	}
}

// TestResolveMultipleBlocks verifies correct block detection with multiple blocks.
func TestResolveMultipleBlocks(t *testing.T) {
	input := "model gpt4 {\n  provider = \"openai\"\n}\nagent researcher {\n  model = gpt4\n  persona = \"Research.\"\n}"
	program := parseProgram(t, input)

	tests := []struct {
		name      string
		line      int
		col       int
		blockName string
	}{
		{"inside model", 2, 3, "gpt4"},
		{"inside agent", 5, 3, "researcher"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := Resolve(program, tt.line, tt.col)
			if ctx.Block == nil {
				t.Fatalf("expected block, got nil")
			}
			if ctx.Block.Name != tt.blockName {
				t.Errorf("Block.Name = %q, want %q", ctx.Block.Name, tt.blockName)
			}
		})
	}
}

// TestResolveSchemaPopulated verifies that the schema is populated
// when the cursor is inside a known block type.
func TestResolveSchemaPopulated(t *testing.T) {
	input := "model gpt4 {\n  provider = \"openai\"\n}"
	program := parseProgram(t, input)

	ctx := Resolve(program, 2, 3)
	if ctx.Schema == nil {
		t.Fatal("Schema should not be nil for model block")
	}
	if _, ok := ctx.Schema.Fields["provider"]; !ok {
		t.Error("Schema should contain 'provider' field")
	}
}

// TestResolveEmptyBlock verifies that an empty block body returns BlockBody.
func TestResolveEmptyBlock(t *testing.T) {
	input := "model gpt4 {\n}"
	program := parseProgram(t, input)

	ctx := Resolve(program, 1, 14)
	if ctx.Position != BlockBody {
		t.Errorf("Position = %v, want BlockBody", ctx.Position)
	}
	if ctx.Block == nil {
		t.Fatal("Block should not be nil")
	}
}

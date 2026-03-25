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

// TestResolveInsideMultiLineString verifies that the cursor inside a
// multi-line string value returns FieldValue, not BlockBody.
func TestResolveInsideMultiLineString(t *testing.T) {
	input := "agent researcher {\n  persona = \"\n    You are a helpful assistant.\n    \"\n}"
	program := parseProgram(t, input)

	// Line 3 is inside the multi-line string value.
	ctx := Resolve(program, 3, 5)
	if ctx.Position != FieldValue {
		t.Errorf("Position = %v, want FieldValue (inside multi-line string)", ctx.Position)
	}
	if ctx.Assignment == nil {
		t.Fatal("Assignment should not be nil")
	}
	if ctx.Assignment.Name != "persona" {
		t.Errorf("Assignment.Name = %q, want %q", ctx.Assignment.Name, "persona")
	}
}

// TestResolveAfterLastAssignment verifies that a new line after the
// last assignment returns BlockBody, not FieldValue.
func TestResolveAfterLastAssignment(t *testing.T) {
	// Line 4 is a blank line between the last assignment and '}'.
	input := "agent researcher {\n  model = \"gpt-4o\"\n  persona = \"You are helpful.\"\n\n}"
	program := parseProgram(t, input)

	ctx := Resolve(program, 4, 1)
	if ctx.Position != BlockBody {
		t.Errorf("Position = %v, want BlockBody (new line after last assignment)", ctx.Position)
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

// TestFindNodeAtBlockName verifies that hovering a block name returns BlockNameNode.
func TestFindNodeAtBlockName(t *testing.T) {
	input := "model gpt4 {\n  provider = \"openai\"\n}"
	program := parseProgram(t, input)

	// "gpt4" starts at col 7.
	node := FindNodeAt(program, 1, 7)
	if node.Kind != BlockNameNode {
		t.Errorf("Kind = %v, want BlockNameNode", node.Kind)
	}
	if node.Block == nil || node.Block.Name != "gpt4" {
		t.Errorf("expected block 'gpt4'")
	}
}

// TestFindNodeAtIdent verifies that hovering an identifier reference returns IdentNode.
func TestFindNodeAtIdent(t *testing.T) {
	input := "model gpt4 {\n  provider = \"openai\"\n}\nagent researcher {\n  model = gpt4\n  persona = \"hi\"\n}"
	program := parseProgram(t, input)

	// "gpt4" reference on line 5, col 11.
	node := FindNodeAt(program, 5, 11)
	if node.Kind != IdentNode {
		t.Errorf("Kind = %v, want IdentNode", node.Kind)
	}
	if node.Ident == nil || node.Ident.Value != "gpt4" {
		t.Errorf("expected ident 'gpt4'")
	}
}

// TestFindNodeAtFieldName verifies that hovering a field key returns FieldNameNode.
func TestFindNodeAtFieldName(t *testing.T) {
	input := "model gpt4 {\n  provider = \"openai\"\n}"
	program := parseProgram(t, input)

	// "provider" on line 2, col 3.
	node := FindNodeAt(program, 2, 3)
	if node.Kind != FieldNameNode {
		t.Errorf("Kind = %v, want FieldNameNode", node.Kind)
	}
	if node.Assignment == nil || node.Assignment.Name != "provider" {
		t.Errorf("expected assignment 'provider'")
	}
}

// TestFindNodeAtIdentInList verifies that identifiers inside list literals are found.
func TestFindNodeAtIdentInList(t *testing.T) {
	input := "tool web_search {\n  provider = \"tavily\"\n}\nagent a {\n  model = \"gpt-4o\"\n  persona = \"hi\"\n  tools = [web_search]\n}"
	program := parseProgram(t, input)

	// "web_search" inside the list on line 7.
	node := FindNodeAt(program, 7, 12)
	if node.Kind != IdentNode {
		t.Errorf("Kind = %v, want IdentNode", node.Kind)
	}
	if node.Ident == nil || node.Ident.Value != "web_search" {
		t.Errorf("expected ident 'web_search', got %v", node.Ident)
	}
}

// TestFindNodeAtMemberAccess verifies that hovering a member name in a
// member access expression (e.g. gpt4.model_name) returns MemberAccessNode.
func TestFindNodeAtMemberAccess(t *testing.T) {
	input := "model gpt4 {\n  provider = \"openai\"\n}\nagent researcher {\n  model = gpt4.provider\n  persona = \"hi\"\n}"
	program := parseProgram(t, input)

	tests := []struct {
		name   string
		line   int
		col    int
		kind   NodeKind
		member string
		ident  string
	}{
		// "gpt4.provider" on line 5: gpt4 starts at col 11, dot at col 15, provider at col 16
		{"on member name", 5, 16, MemberAccessNode, "provider", ""},
		{"on object ident", 5, 11, IdentNode, "", "gpt4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := FindNodeAt(program, tt.line, tt.col)
			if node.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", node.Kind, tt.kind)
			}
			if tt.member != "" && (node.MemberAccess == nil || node.MemberAccess.Member != tt.member) {
				t.Errorf("expected member %q", tt.member)
			}
			if tt.ident != "" && (node.Ident == nil || node.Ident.Value != tt.ident) {
				t.Errorf("expected ident %q", tt.ident)
			}
		})
	}
}

// TestFindNodeAtNone verifies that positions on literals return NoneNode.
func TestFindNodeAtNone(t *testing.T) {
	input := "model gpt4 {\n  provider = \"openai\"\n}"
	program := parseProgram(t, input)

	// On the string literal "openai" — not an ident.
	node := FindNodeAt(program, 2, 16)
	if node.Kind != NoneNode {
		t.Errorf("Kind = %v, want NoneNode for string literal", node.Kind)
	}
}

// TestFindNodeAtNilProgram verifies graceful handling of nil program.
func TestFindNodeAtNilProgram(t *testing.T) {
	node := FindNodeAt(nil, 1, 1)
	if node.Kind != NoneNode {
		t.Errorf("Kind = %v, want NoneNode for nil program", node.Kind)
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

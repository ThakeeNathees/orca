package cursor

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
	"github.com/thakee/orca/compiler/token"
)

// parseProgram is a test helper that parses input and fails on parse errors.
func parseProgram(t *testing.T, input string) *ast.Program {
	t.Helper()
	l := lexer.New(input, "")
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
		blockKind token.BlockKind
		blockName string
	}{
		{"start of field line", 2, 1, BlockBody, token.BlockModel, "gpt4"},
		{"on closing brace line", 3, 1, BlockBody, token.BlockModel, "gpt4"},
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
			if ctx.BlockKind != tt.blockKind {
				t.Errorf("BlockKind = %v, want %v", ctx.BlockKind, tt.blockKind)
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

// TestResolveInsideRawString verifies that the cursor inside a
// raw string value returns FieldValue, not BlockBody.
func TestResolveInsideRawString(t *testing.T) {
	input := "agent researcher {\n  persona = ```\n    You are a helpful assistant.\n  ```\n}"
	program := parseProgram(t, input)

	// Line 3 is inside the raw string value.
	ctx := Resolve(program, 3, 5)
	if ctx.Position != FieldValue {
		t.Errorf("Position = %v, want FieldValue (inside raw string)", ctx.Position)
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

// TestFindNodeAtBinaryExpression verifies that identifiers inside binary
// expressions (e.g. a -> b) are found via findInExpr.
func TestFindNodeAtBinaryExpression(t *testing.T) {
	// graph = a -> a produces BinaryExpression with ident nodes.
	// Line layout:
	// 1: model gpt4 {
	// 2:   provider = "openai"
	// 3: }
	// 4: agent a {
	// 5:   model = "gpt-4o"
	// 6:   persona = "hi"
	// 7: }
	// 8: workflow w {
	// 9:   name = "test"
	// 10:  graph = a -> a
	// 11: }
	input := "model gpt4 {\n  provider = \"openai\"\n}\nagent a {\n  model = \"gpt-4o\"\n  persona = \"hi\"\n}\nworkflow w {\n  name = \"test\"\n  graph = a -> a\n}"
	program := parseProgram(t, input)

	tests := []struct {
		name  string
		line  int
		col   int
		kind  NodeKind
		ident string
	}{
		// "graph = a -> a": graph at col 3, a at col 11, -> at col 13, a at col 16
		{"left of arrow", 10, 11, IdentNode, "a"},
		{"right of arrow", 10, 16, IdentNode, "a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := FindNodeAt(program, tt.line, tt.col)
			if node.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", node.Kind, tt.kind)
			}
			if node.Ident == nil || node.Ident.Value != tt.ident {
				t.Errorf("expected ident %q", tt.ident)
			}
		})
	}
}

// TestFindNodeAtWorkflowEdge verifies that identifiers inside bare workflow
// edge expressions (A -> B -> C) are found via findInExpr.
func TestFindNodeAtWorkflowEdge(t *testing.T) {
	// Line layout:
	// 1: agent A {
	// 2:   model = "gpt-4o"
	// 3:   persona = "hi"
	// 4: }
	// 5: agent B {
	// 6:   model = "gpt-4o"
	// 7:   persona = "hi"
	// 8: }
	// 9: agent C {
	// 10:  model = "gpt-4o"
	// 11:  persona = "hi"
	// 12: }
	// 13: workflow run {
	// 14:   A -> B -> C
	// 15:   C -> A
	// 16: }
	input := "agent A {\n  model = \"gpt-4o\"\n  persona = \"hi\"\n}\nagent B {\n  model = \"gpt-4o\"\n  persona = \"hi\"\n}\nagent C {\n  model = \"gpt-4o\"\n  persona = \"hi\"\n}\nworkflow run {\n  A -> B -> C\n  C -> A\n}"
	program := parseProgram(t, input)

	tests := []struct {
		name  string
		line  int
		col   int
		kind  NodeKind
		ident string
	}{
		// Line 14: "  A -> B -> C"
		// A at col 3, B at col 8, C at col 13
		{"first node in chain", 14, 3, IdentNode, "A"},
		{"middle node in chain", 14, 8, IdentNode, "B"},
		{"last node in chain", 14, 13, IdentNode, "C"},
		// Line 15: "  C -> A"
		// C at col 3, A at col 8
		{"first node second edge", 15, 3, IdentNode, "C"},
		{"last node second edge", 15, 8, IdentNode, "A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := FindNodeAt(program, tt.line, tt.col)
			if node.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", node.Kind, tt.kind)
			}
			if node.Ident == nil || node.Ident.Value != tt.ident {
				t.Errorf("expected ident %q, got %v", tt.ident, node.Ident)
			}
			if node.Block == nil || node.Block.Name != "run" {
				t.Errorf("expected enclosing block 'run'")
			}
		})
	}
}

// TestFindNodeAtWorkflowEdgeNone verifies that the arrow operator itself
// returns NoneNode (not an identifier).
func TestFindNodeAtWorkflowEdgeNone(t *testing.T) {
	input := "agent A {\n  model = \"gpt-4o\"\n  persona = \"hi\"\n}\nagent B {\n  model = \"gpt-4o\"\n  persona = \"hi\"\n}\nworkflow run {\n  A -> B\n}"
	program := parseProgram(t, input)

	// "->" on line 10 at col 5
	node := FindNodeAt(program, 10, 5)
	if node.Kind != NoneNode {
		t.Errorf("Kind = %v, want NoneNode for arrow operator", node.Kind)
	}
}

// TestFindNodeAtSubscription verifies that identifiers inside subscription
// expressions (e.g. items[0]) are found via findInExpr.
func TestFindNodeAtSubscription(t *testing.T) {
	// web_search[0] produces Subscription with ident "web_search" as object.
	// Line layout:
	// 4: let vars {
	// 5:   items = web_search[0]
	// 6: }
	input := "tool web_search {\n  provider = \"tavily\"\n}\nlet vars {\n  items = web_search[0]\n}"
	program := parseProgram(t, input)

	// "web_search" starts at col 11 on line 5
	node := FindNodeAt(program, 5, 11)
	if node.Kind != IdentNode {
		t.Errorf("Kind = %v, want IdentNode (subscription object)", node.Kind)
	}
	if node.Ident == nil || node.Ident.Value != "web_search" {
		t.Errorf("expected ident 'web_search'")
	}
}

// TestFindNodeAtMapLiteral verifies that identifiers inside map literal
// values are found via findInExpr.
func TestFindNodeAtMapLiteral(t *testing.T) {
	// config = {"key": gpt4} produces MapLiteral with ident "gpt4" as a value.
	input := "model gpt4 {\n  provider = \"openai\"\n}\nlet vars {\n  config = {\"key\": gpt4}\n}"
	program := parseProgram(t, input)

	// "gpt4" in the map value on line 5.
	// config = {"key": gpt4}
	// col:  3         15   20
	node := FindNodeAt(program, 5, 21)
	if node.Kind != IdentNode {
		t.Errorf("Kind = %v, want IdentNode (map value)", node.Kind)
	}
	if node.Ident == nil || node.Ident.Value != "gpt4" {
		t.Errorf("expected ident 'gpt4'")
	}
}

// TestFindNodeAtCallExpression verifies that identifiers inside call
// expressions (callee and args) are found via findInExpr.
func TestFindNodeAtCallExpression(t *testing.T) {
	// result = gpt4(gpt4) produces CallExpression.
	input := "model gpt4 {\n  provider = \"openai\"\n}\nlet vars {\n  result = gpt4(gpt4)\n}"
	program := parseProgram(t, input)

	tests := []struct {
		name  string
		line  int
		col   int
		kind  NodeKind
		ident string
	}{
		// "result = gpt4(gpt4)": gpt4 callee at col 12, arg gpt4 at col 17
		{"callee", 5, 12, IdentNode, "gpt4"},
		{"argument", 5, 17, IdentNode, "gpt4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := FindNodeAt(program, tt.line, tt.col)
			if node.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", node.Kind, tt.kind)
			}
			if node.Ident == nil || node.Ident.Value != tt.ident {
				t.Errorf("expected ident %q", tt.ident)
			}
		})
	}
}

// TestFindNodeAtDotCompletion verifies that placing the cursor right after
// a dot triggers DotCompletion mode.
func TestFindNodeAtDotCompletion(t *testing.T) {
	input := "model gpt4 {\n  provider = \"openai\"\n}\nagent researcher {\n  model = gpt4.provider\n  persona = \"hi\"\n}"
	program := parseProgram(t, input)

	// Dot is at col 15, cursor right after dot is col 16.
	node := FindNodeAt(program, 5, 16)
	if node.Kind != MemberAccessNode {
		t.Errorf("Kind = %v, want MemberAccessNode", node.Kind)
	}
}

// TestResolveNilProgram verifies that Resolve handles nil program gracefully.
func TestResolveNilProgram(t *testing.T) {
	ctx := Resolve(nil, 1, 1)
	if ctx.Position != TopLevel {
		t.Errorf("Position = %v, want TopLevel for nil program", ctx.Position)
	}
}

// TestResolveUserSchemaBlock verifies that Resolve inside a user-defined
// schema block looks up the schema by block name.
func TestResolveUserSchemaBlock(t *testing.T) {
	input := "schema vpc_data_t {\n  region = str\n  count = int\n\n}"
	program := parseProgram(t, input)

	// Line 4 is a blank line inside the block — BlockBody position.
	ctx := Resolve(program, 4, 1)
	if ctx.Position != BlockBody {
		t.Errorf("Position = %v, want BlockBody", ctx.Position)
	}
	if ctx.Block == nil {
		t.Fatal("Block should not be nil")
	}
	if ctx.BlockKind != token.BlockSchema {
		t.Errorf("BlockKind = %v, want BlockSchema", ctx.BlockKind)
	}
}

// TestResolveInlineBlockBody verifies that positions inside an inline block
// expression return BlockBody with the inline block's schema.
func TestResolveInlineBlockBody(t *testing.T) {
	input := "agent researcher {\n  model = model {\n    provider = \"openai\"\n\n  }\n\n  persona = \"hi\"\n}"
	program := parseProgram(t, input)

	tests := []struct {
		name      string
		line      int
		col       int
		expect    CursorPosition
		blockKind token.BlockKind
		isInline  bool
	}{
		// Line 4 is blank inside the inline model block.
		{"inside inline block", 4, 3, BlockBody, token.BlockModel, true},
		// Line 6 is blank after the inline block, still inside the agent block.
		{"after inline block", 6, 3, BlockBody, token.BlockAgent, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := Resolve(program, tt.line, tt.col)
			if ctx.Position != tt.expect {
				t.Errorf("Position = %v, want %v", ctx.Position, tt.expect)
			}
			if ctx.BlockKind != tt.blockKind {
				t.Errorf("BlockKind = %v, want %v", ctx.BlockKind, tt.blockKind)
			}
			if tt.isInline {
				if ctx.InlineBlock == nil {
					t.Fatal("InlineBlock should not be nil")
				}
			} else {
				if ctx.InlineBlock != nil {
					t.Error("InlineBlock should be nil")
				}
			}
		})
	}
}

// TestResolveInlineSchemaBody verifies completions inside inline schema blocks.
func TestResolveInlineSchemaBody(t *testing.T) {
	input := "agent researcher {\n  model = \"gpt-4o\"\n  persona = \"hi\"\n  output = schema {\n    name = str\n\n  }\n}"
	program := parseProgram(t, input)

	// Line 6 is blank inside the inline schema block.
	ctx := Resolve(program, 6, 3)
	if ctx.Position != BlockBody {
		t.Errorf("Position = %v, want BlockBody", ctx.Position)
	}
	if ctx.InlineBlock == nil {
		t.Fatal("InlineBlock should not be nil")
	}
	if ctx.BlockKind != token.BlockSchema {
		t.Errorf("BlockKind = %v, want BlockSchema", ctx.BlockKind)
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

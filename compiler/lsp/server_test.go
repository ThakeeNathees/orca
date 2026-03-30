package lsp

import (
	"os"
	"path/filepath"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/cursor"
	"github.com/thakee/orca/compiler/diagnostic"
)

// diagnoseText is a test helper that parses text and returns LSP diagnostics.
func diagnoseText(text string) []protocol.Diagnostic {
	doc := updateDocument("test://diag.oc", text)
	lspDiags := make([]protocol.Diagnostic, 0, len(doc.Diagnostics))
	for _, d := range doc.Diagnostics {
		lspDiags = append(lspDiags, toLspDiagnostic(d))
	}
	return lspDiags
}

func TestDiagnoseValidSource(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"model block", `model gpt4 { provider = "openai" model_name = "gpt-4o" }`},
		{"agent block", `agent a { model = "gpt-4o" persona = "hi" }`},
		{"multiple blocks", "model m { provider = \"openai\" model_name = \"gpt-4o\" }\nagent a { model = m persona = \"hi\" }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := diagnoseText(tt.input)
			if len(diags) != 0 {
				t.Errorf("expected no diagnostics, got %d: %v", len(diags), diags)
			}
		})
	}
}

func TestDiagnoseInvalidSource(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minDiags int
	}{
		{"missing block name", `model { provider = "openai" }`, 1},
		{"missing equals", `model m { provider "openai" }`, 1},
		{"missing value", `model m { provider = }`, 1},
		{"missing closing brace", `model m { provider = "openai"`, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := diagnoseText(tt.input)
			if len(diags) < tt.minDiags {
				t.Errorf("expected at least %d diagnostics, got %d", tt.minDiags, len(diags))
			}
		})
	}
}

func TestDiagnoseSeverityIsError(t *testing.T) {
	diags := diagnoseText(`model { }`)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics")
	}
	if diags[0].Severity == nil || *diags[0].Severity != protocol.DiagnosticSeverityError {
		t.Error("expected error severity")
	}
}

func TestDiagnoseSourceIsParser(t *testing.T) {
	diags := diagnoseText(`model { }`)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics")
	}
	if diags[0].Source == nil || *diags[0].Source != "parser" {
		t.Errorf("expected source 'parser', got %v", diags[0].Source)
	}
}

func TestDiagnoseErrorsClearAfterFix(t *testing.T) {
	diags := diagnoseText(`model { }`)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics for broken source")
	}

	diags = diagnoseText(`model m { provider = "openai" model_name = "gpt-4o" }`)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics after fix, got %d", len(diags))
	}
}

func TestToLspDiagnostic(t *testing.T) {
	d := diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Position: diagnostic.Position{Line: 3, Column: 5},
		Message:  "expected }",
		Source:   "parser",
	}

	lspDiag := toLspDiagnostic(d)

	// LSP is 0-based, so line 3 col 5 becomes line 2 col 4.
	if lspDiag.Range.Start.Line != 2 {
		t.Errorf("expected line 2, got %d", lspDiag.Range.Start.Line)
	}
	if lspDiag.Range.Start.Character != 4 {
		t.Errorf("expected col 4, got %d", lspDiag.Range.Start.Character)
	}
	if lspDiag.Message != "expected }" {
		t.Errorf("expected message 'expected }', got %q", lspDiag.Message)
	}
}

func TestToLspSeverity(t *testing.T) {
	tests := []struct {
		in  diagnostic.Severity
		out protocol.DiagnosticSeverity
	}{
		{diagnostic.Error, protocol.DiagnosticSeverityError},
		{diagnostic.Warning, protocol.DiagnosticSeverityWarning},
		{diagnostic.Info, protocol.DiagnosticSeverityInformation},
		{diagnostic.Hint, protocol.DiagnosticSeverityHint},
	}

	for _, tt := range tests {
		got := toLspSeverity(tt.in)
		if got != tt.out {
			t.Errorf("toLspSeverity(%d) = %d, want %d", tt.in, got, tt.out)
		}
	}
}

// TestCompleteFieldNames verifies that completion inside a block body
// suggests missing schema fields.
func TestCompleteFieldNames(t *testing.T) {
	text := "model gpt4 {\n  provider = \"openai\"\n}"
	doc := updateDocument("test://file.oc", text)

	// Line 2, col 1 — inside block body, should suggest missing fields.
	ctx := resolveAtDocPosition(doc, 1, 0) // 0-based: line 1, char 0
	items := completionItems(ctx)

	// model has 5 fields: provider (already assigned), model_name, temperature, api_key, base_url.
	// Should suggest all except provider.
	if len(items) != 4 {
		t.Fatalf("expected 4 completion items, got %d", len(items))
	}

	labels := make(map[string]bool)
	for _, item := range items {
		labels[item.Label] = true
	}
	if labels["provider"] {
		t.Error("should not suggest already-assigned field 'provider'")
	}
	for _, expected := range []string{"model_name", "temperature", "api_key", "base_url"} {
		if !labels[expected] {
			t.Errorf("should suggest %q", expected)
		}
	}
}

// TestCompleteFieldNamesRequired verifies that required fields sort before optional.
func TestCompleteFieldNamesRequired(t *testing.T) {
	text := "agent researcher {\n}"
	doc := updateDocument("test://file.oc", text)

	ctx := resolveAtDocPosition(doc, 0, 19) // inside empty block
	items := completionItems(ctx)

	// Check that required fields have sort text starting with "0_".
	for _, item := range items {
		if item.SortText == nil {
			t.Errorf("item %q missing SortText", item.Label)
			continue
		}
		if item.Label == "model" || item.Label == "persona" {
			if (*item.SortText)[0] != '0' {
				t.Errorf("required field %q should sort first, got %q", item.Label, *item.SortText)
			}
		}
		if item.Label == "tools" {
			if (*item.SortText)[0] != '1' {
				t.Errorf("optional field %q should sort after required, got %q", item.Label, *item.SortText)
			}
		}
	}
}

// TestCompleteFieldNamesInsertText verifies that completion inserts "field = ".
func TestCompleteFieldNamesInsertText(t *testing.T) {
	text := "model gpt4 {\n}"
	doc := updateDocument("test://file.oc", text)

	ctx := resolveAtDocPosition(doc, 0, 13) // inside empty block
	items := completionItems(ctx)

	for _, item := range items {
		if item.InsertText == nil {
			t.Errorf("item %q missing InsertText", item.Label)
			continue
		}
		expected := item.Label + " = "
		if *item.InsertText != expected {
			t.Errorf("item %q InsertText = %q, want %q", item.Label, *item.InsertText, expected)
		}
	}
}

// findMemberCompletion is a test helper that finds a dot-completion position
// at the given 0-based position and returns completion items for it.
func findMemberCompletion(doc *documentState, line, char int) []protocol.CompletionItem {
	node := cursor.FindNodeAt(doc.Program, line+1, char+1)
	if node.Kind != cursor.MemberAccessNode || !node.DotCompletion {
		return nil
	}
	return completeMemberFields(doc, node.MemberAccess)
}

// TestCompleteMemberAccess verifies that typing "gpt4." suggests all fields
// of the model block.
func TestCompleteMemberAccess(t *testing.T) {
	text := "model gpt4 {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent researcher {\n  model = gpt4.\n  persona = \"hi\"\n}"
	doc := updateDocument("test://dotcomp.oc", text)

	// Cursor is right after "gpt4." on line 6 (0-based: line 5, char 15).
	items := findMemberCompletion(doc, 5, 15)
	if len(items) == 0 {
		t.Fatal("expected completion items after 'gpt4.'")
	}

	labels := make(map[string]bool)
	for _, item := range items {
		labels[item.Label] = true
	}
	// model schema has: provider, model_name, temperature.
	if !labels["provider"] {
		t.Error("should suggest 'provider'")
	}
	if !labels["model_name"] {
		t.Error("should suggest 'model_name'")
	}
	if !labels["temperature"] {
		t.Error("should suggest 'temperature'")
	}
}

// TestCompleteMemberAccessKind verifies that member completions use Property kind.
func TestCompleteMemberAccessKind(t *testing.T) {
	text := "model gpt4 {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent researcher {\n  model = gpt4.\n  persona = \"hi\"\n}"
	doc := updateDocument("test://dotcomp2.oc", text)

	items := findMemberCompletion(doc, 5, 15)
	for _, item := range items {
		if item.Kind == nil || *item.Kind != protocol.CompletionItemKindProperty {
			t.Errorf("item %q should have Property kind", item.Label)
		}
	}
}

// TestCompleteMemberAccessNoResults verifies no completions for unknown identifiers.
func TestCompleteMemberAccessNoResults(t *testing.T) {
	text := "agent researcher {\n  model = unknown.\n  persona = \"hi\"\n}"
	doc := updateDocument("test://dotcomp3.oc", text)

	items := findMemberCompletion(doc, 1, 18)
	if len(items) != 0 {
		t.Errorf("expected no completions for unknown block, got %d", len(items))
	}
}

// TestCompleteMemberAccessPrimitiveInput verifies that dot-completing on an input
// block with a primitive type (e.g. str) returns no completions instead of
// falling through to the enclosing block's field suggestions.
func TestCompleteMemberAccessPrimitiveInput(t *testing.T) {
	// "dev_vars." with a following field causes the parser to recover a
	// MemberAccess node, matching the real editing scenario.
	text := "input dev_vars {\n  type = str\n}\n\nmodel my_model {\n  provider = dev_vars.\n  model_name = \"gpt-4o\"\n}"
	doc := updateDocument("test://dotcomp_input.oc", text)

	// "dev_vars." — the dot is recovered as a MemberAccess.
	// completeMemberFields should return nil (str has no schema fields),
	// and we must NOT fall through to model block body completions.
	items := findMemberCompletion(doc, 5, 22)
	if len(items) != 0 {
		t.Errorf("expected no completions for primitive-typed input dot-access, got %d", len(items))
		for _, item := range items {
			t.Errorf("  unexpected: %s", item.Label)
		}
	}
}

// TestCompleteMemberAccessDescription verifies that completions include field descriptions.
func TestCompleteMemberAccessDescription(t *testing.T) {
	text := "model gpt4 {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent researcher {\n  model = gpt4.\n  persona = \"hi\"\n}"
	doc := updateDocument("test://dotcomp4.oc", text)

	items := findMemberCompletion(doc, 5, 15)
	for _, item := range items {
		if item.Detail == nil || *item.Detail == "" {
			t.Errorf("item %q should have a type detail", item.Label)
		}
	}
}

// TestHoverOnBlockName verifies that hovering a block name shows its type.
func TestHoverOnBlockName(t *testing.T) {
	text := "model gpt4 {\n  provider = \"openai\"\n}"
	doc := updateDocument("test://hover.oc", text)

	node := findNodeAtDoc(doc, 0, 6) // 0-based: "gpt4" starts at char 6
	if node.Kind != cursor.BlockNameNode {
		t.Fatalf("Kind = %v, want BlockNameNode", node.Kind)
	}
}

// TestHoverOnIdentReference verifies hover on an identifier reference.
func TestHoverOnIdentReference(t *testing.T) {
	text := "model gpt4 {\n  provider = \"openai\"\n}\nagent researcher {\n  model = gpt4\n  persona = \"hi\"\n}"
	doc := updateDocument("test://hover.oc", text)

	// "gpt4" reference on line 4 (0-based), char 10.
	node := findNodeAtDoc(doc, 4, 10)
	if node.Kind != cursor.IdentNode {
		t.Fatalf("Kind = %v, want IdentNode", node.Kind)
	}
	if node.Ident.Value != "gpt4" {
		t.Errorf("Ident.Value = %q, want %q", node.Ident.Value, "gpt4")
	}

	// Verify symbol lookup works.
	sym, found := doc.Symbols.LookupSymbol("gpt4")
	if !found {
		t.Fatal("expected gpt4 in symbol table")
	}
	if sym.DefToken.Line != 1 {
		t.Errorf("DefToken.Line = %d, want 1", sym.DefToken.Line)
	}
}

// TestHoverOnFieldName verifies hover on a field key.
func TestHoverOnFieldName(t *testing.T) {
	text := "model gpt4 {\n  provider = \"openai\"\n}"
	doc := updateDocument("test://hover.oc", text)

	// "provider" on line 1 (0-based), char 2.
	node := findNodeAtDoc(doc, 1, 2)
	if node.Kind != cursor.FieldNameNode {
		t.Fatalf("Kind = %v, want FieldNameNode", node.Kind)
	}
	if node.Assignment.Name != "provider" {
		t.Errorf("Assignment.Name = %q, want %q", node.Assignment.Name, "provider")
	}
}

// TestDefinitionJumpsToBlock verifies that go-to-definition on an ident
// reference jumps to the block's name token.
func TestDefinitionJumpsToBlock(t *testing.T) {
	text := "model gpt4 {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent researcher {\n  model = gpt4\n  persona = \"hi\"\n}"
	doc := updateDocument("test://def.oc", text)

	// "gpt4" reference on line 6 (1-based), col 11.
	loc, found := resolveDefinition(doc, 6, 11)
	if !found {
		t.Fatal("expected definition location")
	}
	// Should jump to line 1, col 7 (1-based) → LSP 0-based: line 0, char 6.
	if loc.Range.Start.Line != 0 || loc.Range.Start.Character != 6 {
		t.Errorf("definition at (%d, %d), want (0, 6)",
			loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

// TestDefinitionMemberJumpsToField verifies that go-to-definition on a
// member access (e.g. gpt4.provider) jumps to the field assignment inside
// the referenced block.
func TestDefinitionMemberJumpsToField(t *testing.T) {
	text := "model gpt4 {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent researcher {\n  model = gpt4.provider\n  persona = \"hi\"\n}"
	doc := updateDocument("test://memdef.oc", text)

	// "provider" member on line 6 (1-based). "gpt4.provider" starts at col 11.
	// "provider" starts at col 16 (after "gpt4.").
	loc, found := resolveDefinition(doc, 6, 16)
	if !found {
		t.Fatal("expected definition location for gpt4.provider")
	}
	// "provider" assignment is on line 2, col 3 → LSP 0-based: line 1, char 2.
	if loc.Range.Start.Line != 1 || loc.Range.Start.Character != 2 {
		t.Errorf("definition at (%d, %d), want (1, 2)",
			loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

// TestDefinitionMemberModelName verifies go-to-definition on gpt4.model_name
// jumps to the model_name field assignment.
func TestDefinitionMemberModelName(t *testing.T) {
	text := "model gpt4 {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent researcher {\n  model = gpt4.model_name\n  persona = \"hi\"\n}"
	doc := updateDocument("test://memdef2.oc", text)

	// "model_name" member on line 6. "gpt4.model_name" starts at col 11.
	// "model_name" starts at col 16 (after "gpt4.").
	loc, found := resolveDefinition(doc, 6, 16)
	if !found {
		t.Fatal("expected definition location for gpt4.model_name")
	}
	// "model_name" assignment is on line 3, col 3 → LSP 0-based: line 2, char 2.
	if loc.Range.Start.Line != 2 || loc.Range.Start.Character != 2 {
		t.Errorf("definition at (%d, %d), want (2, 2)",
			loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

// TestDefinitionMemberUnassignedField verifies that go-to-definition on a
// schema field that isn't assigned falls back to the block name.
func TestDefinitionMemberUnassignedField(t *testing.T) {
	text := "model gpt4 {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent researcher {\n  model = gpt4.temperature\n  persona = \"hi\"\n}"
	doc := updateDocument("test://memdef3.oc", text)

	// "temperature" is a valid schema field for model but not assigned.
	// "gpt4.temperature" — "temperature" starts at col 16.
	loc, found := resolveDefinition(doc, 6, 16)
	if !found {
		t.Fatal("expected fallback definition location for unassigned schema field")
	}
	// Should fall back to block name "gpt4" at line 1, col 7 → LSP: line 0, char 6.
	if loc.Range.Start.Line != 0 || loc.Range.Start.Character != 6 {
		t.Errorf("definition at (%d, %d), want (0, 6)",
			loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

// TestDefinitionMemberInvalidField verifies that go-to-definition on a
// member that doesn't exist in the block or schema returns nothing.
func TestDefinitionMemberInvalidField(t *testing.T) {
	text := "model gpt4 {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent researcher {\n  model = gpt4.nonexistent\n  persona = \"hi\"\n}"
	doc := updateDocument("test://memdef4.oc", text)

	// "nonexistent" is not a schema field and not assigned.
	loc, found := resolveDefinition(doc, 6, 16)
	if found {
		t.Errorf("expected no definition for nonexistent field, got (%d, %d)",
			loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

// TestDefinitionMemberChained verifies go-to-definition through chained
// member access (e.g. researcher.model resolves to the model block,
// then researcher.model.provider goes to that model's provider field).
func TestDefinitionMemberChained(t *testing.T) {
	text := "model gpt4 {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent researcher {\n  model = gpt4\n  persona = \"hi\"\n}\n@suppress(\"type-mismatch\")\nagent consumer {\n  model = researcher.model\n  persona = \"hi\"\n}"
	doc := updateDocument("test://chain.oc", text)

	// "researcher.model" — "model" starts at col 23 on line 11 (1-based).
	loc, found := resolveDefinition(doc, 11, 23)
	if !found {
		t.Fatal("expected definition location for researcher.model")
	}
	// "model" assignment in researcher block at line 6, col 3 → LSP: line 5, char 2.
	if loc.Range.Start.Line != 5 || loc.Range.Start.Character != 2 {
		t.Errorf("definition at (%d, %d), want (5, 2)",
			loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

// TestDefinitionSchemaField verifies that go-to-definition on a member of an
// inline schema expression jumps to the field assignment inside the schema.
// e.g. cursor on "draft" in "researcher.output.draft" → schema { draft = str }.
func TestDefinitionSchemaField(t *testing.T) {
	text := "agent researcher {\n  persona = \"hi\"\n  output_schema = schema {\n    draft = str\n    score = int\n  }\n}\n@suppress(\"type-mismatch\")\nagent consumer {\n  persona = researcher.output_schema.draft\n  model = \"x\"\n}"
	doc := updateDocument("test://schema-def.oc", text)

	// "draft" in "researcher.output_schema.draft" on line 10, col 38 (1-based).
	loc, found := resolveDefinition(doc, 10, 38)
	if !found {
		t.Fatal("expected definition for researcher.output_schema.draft")
	}
	// "draft = str" is at line 4, col 5 → LSP: line 3, char 4.
	if loc.Range.Start.Line != 3 || loc.Range.Start.Character != 4 {
		t.Errorf("definition at (%d, %d), want (3, 4)",
			loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

// TestDefinitionSchemaFieldScore verifies go-to-definition on a second field
// in an inline schema to make sure it's not always matching the first field.
func TestDefinitionSchemaFieldScore(t *testing.T) {
	text := "agent researcher {\n  persona = \"hi\"\n  output_schema = schema {\n    draft = str\n    score = int\n  }\n}\n@suppress(\"type-mismatch\")\nagent consumer {\n  persona = researcher.output_schema.score\n  model = \"x\"\n}"
	doc := updateDocument("test://schema-def2.oc", text)

	// "score" in "researcher.output_schema.score" on line 10, col 38 (1-based).
	loc, found := resolveDefinition(doc, 10, 38)
	if !found {
		t.Fatal("expected definition for researcher.output_schema.score")
	}
	// "score = int" is at line 5, col 5 → LSP: line 4, char 4.
	if loc.Range.Start.Line != 4 || loc.Range.Start.Character != 4 {
		t.Errorf("definition at (%d, %d), want (4, 4)",
			loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

// TestDefinitionSchemaUnknownField verifies that go-to-definition returns
// nothing for a member that doesn't exist in the inline schema.
func TestDefinitionSchemaUnknownField(t *testing.T) {
	text := "agent researcher {\n  persona = \"hi\"\n  output = schema {\n    draft = str\n  }\n}\ntask t1 {\n  agent = researcher\n  prompt = researcher.output.missing\n}"
	doc := updateDocument("test://schema-def3.oc", text)

	_, found := resolveDefinition(doc, 9, 30)
	if found {
		t.Error("expected no definition for researcher.output.missing")
	}
}

// TestDefinitionInputBlock verifies that go-to-definition on an input block
// reference jumps to the input block definition.
func TestDefinitionInputBlock(t *testing.T) {
	text := "input topic {\n  type = schema {\n    name = str\n  }\n  desc = \"The topic\"\n}\nagent researcher {\n  persona = topic\n}"
	doc := updateDocument("test://input-def.oc", text)

	// "topic" on line 8, col 13.
	loc, found := resolveDefinition(doc, 8, 13)
	if !found {
		t.Fatal("expected definition for topic reference")
	}
	// "input topic" — name token at line 1, col 7 → LSP: line 0, char 6.
	if loc.Range.Start.Line != 0 || loc.Range.Start.Character != 6 {
		t.Errorf("definition at (%d, %d), want (0, 6)",
			loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

// TestDefinitionInputSchemaField verifies that go-to-definition on a member
// of an input block's type schema jumps to the field inside the schema.
func TestDefinitionInputSchemaField(t *testing.T) {
	text := "input topic {\n  type = schema {\n    name = str\n    tags = list\n  }\n  desc = \"The topic\"\n}\nagent researcher {\n  persona = topic.type.name\n}"
	doc := updateDocument("test://input-schema-def.oc", text)

	// "name" in "topic.type.name" on line 9.
	loc, found := resolveDefinition(doc, 9, 24)
	if !found {
		t.Fatal("expected definition for topic.type.name")
	}
	// "name = str" at line 3, col 5 → LSP: line 2, char 4.
	if loc.Range.Start.Line != 2 || loc.Range.Start.Character != 4 {
		t.Errorf("definition at (%d, %d), want (2, 4)",
			loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

// TestDefinitionInputDirectMember verifies that go-to-definition on a direct
// member of an input block resolves through the input's type schema.
// e.g. some_input.model_name → type = schema { model_name = str }.
func TestDefinitionInputDirectMember(t *testing.T) {
	text := "input some_input {\n  type = schema {\n    model_name = str\n  }\n}\nmodel some_model {\n  model_name = some_input.model_name\n  provider = \"openai\"\n}"
	doc := updateDocument("test://input-direct.oc", text)

	// "model_name" in "some_input.model_name" on line 7.
	// some_input ends at col 25, dot at 26, model_name starts at col 27.
	loc, found := resolveDefinition(doc, 7, 27)
	if !found {
		t.Fatal("expected definition for some_input.model_name")
	}
	// "model_name = str" at line 3, col 5 → LSP: line 2, char 4.
	if loc.Range.Start.Line != 2 || loc.Range.Start.Character != 4 {
		t.Errorf("definition at (%d, %d), want (2, 4)",
			loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

// TestDefinitionNoSymbols verifies graceful handling when symbols are nil
// (e.g. parse errors prevent analysis).
func TestDefinitionNoSymbols(t *testing.T) {
	doc := &documentState{Program: &ast.Program{}, Symbols: nil}
	_, found := resolveDefinition(doc, 1, 1)
	if found {
		t.Error("expected no definition when symbols are nil")
	}
}

// --- Crash prevention tests ---
// These test partial/broken source that the user types mid-edit. The LSP must
// never panic on any of these inputs — it should process them gracefully
// (returning diagnostics, empty completions, etc.) without crashing.

// TestNoCrashIncompleteMemberAccess verifies the analyzer doesn't crash on "gpt4."
// where the member name is missing.
func TestNoCrashIncompleteMemberAccess(t *testing.T) {
	inputs := []struct {
		name  string
		input string
	}{
		{"dot at end of field", "model gpt4 {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent a {\n  model = gpt4.\n  persona = \"hi\"\n}"},
		{"dot at end of file", "model gpt4 {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent a {\n  model = gpt4."},
		{"double dot", "model gpt4 {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent a {\n  model = gpt4..\n  persona = \"hi\"\n}"},
		{"dot then brace", "model gpt4 {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent a {\n  model = gpt4.}"},
	}

	for _, tt := range inputs {
		t.Run(tt.name, func(t *testing.T) {
			// Must not panic.
			_ = updateDocument("test://crash-member.oc", tt.input)
		})
	}
}

// TestNoCrashIncompleteSubscription verifies the analyzer doesn't crash on
// incomplete subscript expressions like "list[", "list[0", "list[]".
func TestNoCrashIncompleteSubscription(t *testing.T) {
	inputs := []struct {
		name  string
		input string
	}{
		{"open bracket only", "agent a {\n  model = i.key[\n  persona = \"hi\"\n}"},
		{"bracket with value no close", "agent a {\n  model = i.key[0\n  persona = \"hi\"\n}"},
		{"empty brackets", "agent a {\n  model = i.key[]\n  persona = \"hi\"\n}"},
		{"nested incomplete", "agent a {\n  model = i.key[foo.\n  persona = \"hi\"\n}"},
		{"bracket at end of file", "agent a {\n  model = i.key["},
		{"subscription then dot", "agent a {\n  model = i.key[0].\n  persona = \"hi\"\n}"},
	}

	for _, tt := range inputs {
		t.Run(tt.name, func(t *testing.T) {
			// Must not panic.
			_ = updateDocument("test://crash-sub.oc", tt.input)
		})
	}
}

// TestNoCrashMalformedExpressions verifies the analyzer doesn't crash on
// various malformed expressions that can appear while the user is typing.
func TestNoCrashMalformedExpressions(t *testing.T) {
	inputs := []struct {
		name  string
		input string
	}{
		{"incomplete binary", "agent a {\n  model = gpt4 +\n  persona = \"hi\"\n}"},
		{"missing value", "agent a {\n  model =\n  persona = \"hi\"\n}"},
		{"missing value at eof", "agent a {\n  model ="},
		{"incomplete list", "agent a {\n  tools = [\n  persona = \"hi\"\n}"},
		{"incomplete map", "agent a {\n  model = {\n  persona = \"hi\"\n}"},
		{"incomplete call", "agent a {\n  model = foo(\n  persona = \"hi\"\n}"},
		{"just an equals", "agent a {\n  =\n}"},
		{"keyword as field", "agent a {\n  model\n}"},
		{"empty block", "agent a {}"},
		{"nested member then bracket", "agent a {\n  model = gpt4.provider[\n  persona = \"hi\"\n}"},
		{"chained dots incomplete", "agent a {\n  model = a.b.c.\n  persona = \"hi\"\n}"},
		{"string in workflow call arg", "workflow research {\n  researcher({\n    \"\"\n  })\n}"},
		{"empty map in workflow call", "workflow research {\n  researcher({\n  })\n}"},
	}

	for _, tt := range inputs {
		t.Run(tt.name, func(t *testing.T) {
			// Must not panic.
			_ = updateDocument("test://crash-expr.oc", tt.input)
		})
	}
}

// TestNoCrashCompletionOnPartialParse verifies that completion requests on
// broken source don't crash.
func TestNoCrashCompletionOnPartialParse(t *testing.T) {
	inputs := []struct {
		name string
		input string
		line int
		char int
	}{
		{"dot completion", "model m {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent a {\n  model = m.\n  persona = \"hi\"\n}", 5, 13},
		{"bracket mid-type", "agent a {\n  model = i.key[\n  persona = \"hi\"\n}", 1, 16},
		{"empty file", "", 0, 0},
		{"just block keyword", "model", 0, 5},
	}

	for _, tt := range inputs {
		t.Run(tt.name, func(t *testing.T) {
			doc := updateDocument("test://crash-comp.oc", tt.input)
			// Must not panic — simulate what the LSP handler does.
			line := tt.line + 1
			col := tt.char + 1
			node := cursor.FindNodeAt(doc.Program, line, col)
			if node.Kind == cursor.MemberAccessNode && node.DotCompletion {
				_ = completeMemberFields(doc, node.MemberAccess)
			}
			cursorCtx := cursor.Resolve(doc.Program, line, col)
			_ = completionItems(cursorCtx)
		})
	}
}

// TestCompletionInsideInlineBlock verifies that field completions work inside
// inline block expressions (e.g. model = model { | }).
func TestCompletionInsideInlineBlock(t *testing.T) {
	input := "agent researcher {\n  model = model {\n    provider = \"openai\"\n\n  }\n  persona = \"hi\"\n}"
	doc := updateDocument("test://inline-comp.oc", input)

	// Line 4 (0-based: 3) is the blank line inside the inline model block.
	line := 4
	col := 3
	cursorCtx := cursor.Resolve(doc.Program, line, col)
	items := completionItems(cursorCtx)

	if len(items) == 0 {
		t.Fatal("expected completions inside inline block, got none")
	}

	// "provider" is already assigned, so it should not appear.
	for _, item := range items {
		if item.Label == "provider" {
			t.Error("provider should be filtered out (already assigned)")
		}
	}

	// "model_name" should be suggested.
	found := false
	for _, item := range items {
		if item.Label == "model_name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected model_name in completions")
	}
}

// TestNoCrashHoverOnPartialParse verifies that hover on broken source doesn't crash.
func TestNoCrashHoverOnPartialParse(t *testing.T) {
	inputs := []struct {
		name  string
		input string
		line  int
		char  int
	}{
		{"incomplete member", "model m {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent a {\n  model = m.\n  persona = \"hi\"\n}", 5, 13},
		{"incomplete subscription", "agent a {\n  model = i.key[\n  persona = \"hi\"\n}", 1, 14},
	}

	for _, tt := range inputs {
		t.Run(tt.name, func(t *testing.T) {
			doc := updateDocument("test://crash-hover.oc", tt.input)
			// Must not panic.
			_ = cursor.FindNodeAt(doc.Program, tt.line+1, tt.char+1)
		})
	}
}

// findNodeAtDoc converts 0-based LSP positions to 1-based and calls FindNodeAt.
func findNodeAtDoc(doc *documentState, line, char int) cursor.NodeAt {
	if doc.Program == nil {
		return cursor.NodeAt{}
	}
	return cursor.FindNodeAt(doc.Program, line+1, char+1)
}

// TestCrossFileReferenceResolution verifies that symbols defined in sibling
// .oc files are visible during analysis. An input block in inputs.oc should
// resolve when referenced from main.oc.
func TestCrossFileReferenceResolution(t *testing.T) {
	tests := []struct {
		name     string
		mainText string
		siblings map[string]string // filename → content (written to same temp dir)
		wantErr  bool              // expect undefined-ref errors in main
	}{
		{
			name:     "cross-file input reference resolves",
			mainText: "model llm {\n  provider = provider\n  model_name = \"gpt-4o\"\n}",
			siblings: map[string]string{
				"inputs.oc": "input provider {\n  type = str\n  desc = \"LLM provider\"\n}",
			},
			wantErr: false,
		},
		{
			name:     "undefined ref without sibling file",
			mainText: "model llm {\n  provider = provider\n  model_name = \"gpt-4o\"\n}",
			siblings: map[string]string{},
			wantErr:  true,
		},
		{
			name:     "multiple cross-file refs",
			mainText: "model llm {\n  provider = prov\n  model_name = mname\n}",
			siblings: map[string]string{
				"inputs.oc": "input prov {\n  type = str\n  desc = \"p\"\n}\ninput mname {\n  type = str\n  desc = \"m\"\n}",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// Write sibling files to disk.
			for name, content := range tt.siblings {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatalf("failed to write %s: %v", name, err)
				}
			}

			// Use a file:// URI pointing to main.oc in the temp dir so
			// mergeSiblingFiles can discover the sibling .oc files.
			mainPath := filepath.Join(dir, "main.oc")
			uri := "file://" + mainPath

			doc := updateDocument(uri, tt.mainText)

			hasUndefinedRef := false
			for _, d := range doc.Diagnostics {
				if d.Code == diagnostic.CodeUndefinedRef {
					hasUndefinedRef = true
					break
				}
			}

			if tt.wantErr && !hasUndefinedRef {
				t.Error("expected undefined-ref diagnostic, got none")
			}
			if !tt.wantErr && hasUndefinedRef {
				t.Error("unexpected undefined-ref diagnostic from cross-file reference")
			}
		})
	}
}

// TestCrossFileGoToDefinition verifies that go-to-definition on a cross-file
// reference returns a location with the sibling file's URI, not the current file.
func TestCrossFileGoToDefinition(t *testing.T) {
	dir := t.TempDir()

	sibContent := "input provider {\n  type = str\n  desc = \"LLM provider\"\n}"
	sibPath := filepath.Join(dir, "inputs.oc")
	if err := os.WriteFile(sibPath, []byte(sibContent), 0644); err != nil {
		t.Fatal(err)
	}

	// "provider" reference is at line 2, col 15 (1-based).
	mainText := "model llm {\n  provider = provider\n  model_name = \"gpt-4o\"\n}"
	mainPath := filepath.Join(dir, "main.oc")
	mainURI := "file://" + mainPath

	doc := updateDocument(mainURI, mainText)

	// Resolve definition for "provider" identifier at 0-based line 1, char 15.
	loc, found := resolveDefinition(doc, 2, 15)
	if !found {
		t.Fatal("expected to find definition for cross-file reference")
	}

	expectedURI := "file://" + sibPath
	if loc.URI != expectedURI {
		t.Errorf("go-to-definition URI = %q, want %q", loc.URI, expectedURI)
	}
	// The "provider" name token is on line 1, col 7 (0-based: line 0, char 6).
	if loc.Range.Start.Line != 0 || loc.Range.Start.Character != 6 {
		t.Errorf("go-to-definition range start = (%d,%d), want (0,6)",
			loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

// resolveAtDocPosition resolves cursor context from 0-based LSP positions.
func resolveAtDocPosition(doc *documentState, line, char int) cursor.Context {
	if doc.Program == nil {
		return cursor.Context{}
	}
	return cursor.Resolve(doc.Program, line+1, char+1)
}

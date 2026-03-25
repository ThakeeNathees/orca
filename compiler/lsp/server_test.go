package lsp

import (
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

	// model has 3 fields: provider (already assigned), model_name, temperature.
	// Should suggest model_name and temperature but not provider.
	if len(items) != 2 {
		t.Fatalf("expected 2 completion items, got %d", len(items))
	}

	labels := make(map[string]bool)
	for _, item := range items {
		labels[item.Label] = true
	}
	if labels["provider"] {
		t.Error("should not suggest already-assigned field 'provider'")
	}
	if !labels["model_name"] {
		t.Error("should suggest 'model_name'")
	}
	if !labels["temperature"] {
		t.Error("should suggest 'temperature'")
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
	text := "model gpt4 {\n  provider = \"openai\"\n  model_name = \"gpt-4o\"\n}\nagent researcher {\n  model = gpt4\n  persona = \"hi\"\n}\ntask t1 {\n  agent = researcher.model\n  action = \"do stuff\"\n}"
	doc := updateDocument("test://chain.oc", text)

	// "researcher.model" — "model" starts at col 22 on line 10.
	loc, found := resolveDefinition(doc, 10, 22)
	if !found {
		t.Fatal("expected definition location for researcher.model")
	}
	// "model" assignment in researcher block at line 6, col 3 → LSP: line 5, char 2.
	if loc.Range.Start.Line != 5 || loc.Range.Start.Character != 2 {
		t.Errorf("definition at (%d, %d), want (5, 2)",
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

// findNodeAtDoc converts 0-based LSP positions to 1-based and calls FindNodeAt.
func findNodeAtDoc(doc *documentState, line, char int) cursor.NodeAt {
	if doc.Program == nil {
		return cursor.NodeAt{}
	}
	return cursor.FindNodeAt(doc.Program, line+1, char+1)
}

// resolveAtDocPosition resolves cursor context from 0-based LSP positions.
func resolveAtDocPosition(doc *documentState, line, char int) cursor.Context {
	if doc.Program == nil {
		return cursor.Context{}
	}
	return cursor.Resolve(doc.Program, line+1, char+1)
}

package lsp

import (
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/thakee/orca/compiler/diagnostic"
)

func TestDiagnoseValidSource(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"model block", `model gpt4 { provider = "openai" }`},
		{"agent block", `agent a { model = gpt4 prompt = "hi" }`},
		{"multiple blocks", "model m {}\nagent a {}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := Diagnose(tt.input)
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
			diags := Diagnose(tt.input)
			if len(diags) < tt.minDiags {
				t.Errorf("expected at least %d diagnostics, got %d", tt.minDiags, len(diags))
			}
		})
	}
}

func TestDiagnoseSeverityIsError(t *testing.T) {
	diags := Diagnose(`model { }`)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics")
	}
	if diags[0].Severity == nil || *diags[0].Severity != protocol.DiagnosticSeverityError {
		t.Error("expected error severity")
	}
}

func TestDiagnoseSourceIsParser(t *testing.T) {
	diags := Diagnose(`model { }`)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics")
	}
	if diags[0].Source == nil || *diags[0].Source != "parser" {
		t.Errorf("expected source 'parser', got %v", diags[0].Source)
	}
}

func TestDiagnoseErrorsClearAfterFix(t *testing.T) {
	diags := Diagnose(`model { }`)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics for broken source")
	}

	diags = Diagnose(`model m { }`)
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

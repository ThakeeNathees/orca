package lsp

import (
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
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
		minDiags int // at least this many diagnostics expected
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

func TestDiagnoseSourceIsOrca(t *testing.T) {
	diags := Diagnose(`model { }`)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics")
	}
	if diags[0].Source == nil || *diags[0].Source != "orca" {
		t.Errorf("expected source 'orca', got %v", diags[0].Source)
	}
}

func TestDiagnoseErrorsClearAfterFix(t *testing.T) {
	// First parse with error.
	diags := Diagnose(`model { }`)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics for broken source")
	}

	// Then parse the fixed version.
	diags = Diagnose(`model m { }`)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics after fix, got %d", len(diags))
	}
}

func TestErrMsgToRange(t *testing.T) {
	tests := []struct {
		name    string
		msg     string
		expLine uint32
		expCol  uint32
	}{
		{"line 1 col 1", "line 1, col 1: something", 0, 0},
		{"line 3 col 5", "line 3, col 5: error", 2, 4},
		{"unparseable", "weird error", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := errMsgToRange(tt.msg)
			if r.Start.Line != protocol.UInteger(tt.expLine) {
				t.Errorf("expected line %d, got %d", tt.expLine, r.Start.Line)
			}
			if r.Start.Character != protocol.UInteger(tt.expCol) {
				t.Errorf("expected col %d, got %d", tt.expCol, r.Start.Character)
			}
		})
	}
}

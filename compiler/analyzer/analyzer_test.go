package analyzer

import (
	"strings"
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/diagnostic"
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

func TestAnalyzeEmptyProgram(t *testing.T) {
	program := &ast.Program{}
	diags := Analyze(program)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics, got %v", diags)
	}
}

// TestAnalyzeMissingRequiredField verifies that the analyzer reports
// an error when a block is missing a required field.
func TestAnalyzeMissingRequiredField(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		fieldName   string
	}{
		{
			"model missing provider",
			`model gpt4 {
				temperature = 0.7
			}`,
			true,
			"provider",
		},
		{
			"model with provider",
			`model gpt4 {
				provider = "openai"
			}`,
			false,
			"",
		},
		{
			"agent missing model",
			`agent researcher {
				persona = "You are a researcher."
			}`,
			true,
			"model",
		},
		{
			"agent missing persona",
			`agent researcher {
				model = "gpt-4o"
			}`,
			true,
			"persona",
		},
		{
			"agent with all required fields",
			`agent researcher {
				model = "gpt-4o"
				persona = "You are a researcher."
			}`,
			false,
			"",
		},
		{
			"task missing agent",
			`task research {
				prompt = "Do research."
			}`,
			true,
			"agent",
		},
		{
			"task missing prompt",
			`task research {
				agent = researcher
			}`,
			true,
			"prompt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			diags := Analyze(program)

			if tt.expectError {
				found := false
				for _, d := range diags {
					if d.Severity == diagnostic.Error && strings.Contains(d.Message, tt.fieldName) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error about missing required field %q, got %v", tt.fieldName, diags)
				}
			} else {
				for _, d := range diags {
					if d.Severity == diagnostic.Error {
						t.Errorf("unexpected error: %s", d.Message)
					}
				}
			}
		})
	}
}

// TestAnalyzeMultipleMissingFields verifies that all missing required
// fields are reported, not just the first one.
func TestAnalyzeMultipleMissingFields(t *testing.T) {
	input := `agent researcher {}`
	program := parseProgram(t, input)
	diags := Analyze(program)

	errorCount := 0
	for _, d := range diags {
		if d.Severity == diagnostic.Error {
			errorCount++
		}
	}
	// agent requires both model and persona
	if errorCount < 2 {
		t.Errorf("expected at least 2 errors for missing model and persona, got %d: %v", errorCount, diags)
	}
}


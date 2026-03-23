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
	diags := Analyze(program).Diagnostics
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
			diags := Analyze(program).Diagnostics

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
	diags := Analyze(program).Diagnostics

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

// TestAnalyzeFieldTypeMismatch verifies that assigning a value of the
// wrong type to a field produces an error.
func TestAnalyzeFieldTypeMismatch(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			"string field with string value",
			`model gpt4 { provider = "openai" }`,
			false,
		},
		{
			"string field with int value",
			`model gpt4 { provider = 42 }`,
			true,
		},
		{
			"float field with float value",
			`model gpt4 { provider = "openai" temperature = 0.7 }`,
			false,
		},
		{
			"float field with string value",
			`model gpt4 { provider = "openai" temperature = "high" }`,
			true,
		},
		{
			"list field with list value",
			`agent a { model = "gpt4" persona = "hi" tools = [web_search] }`,
			false,
		},
		{
			"union field accepts string",
			`agent a { model = "gpt-4o" persona = "hi" }`,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			diags := Analyze(program).Diagnostics

			hasTypeError := false
			for _, d := range diags {
				if d.Severity == diagnostic.Error && strings.Contains(d.Message, "expects type") {
					hasTypeError = true
					break
				}
			}
			if tt.expectError && !hasTypeError {
				t.Errorf("expected type mismatch error, got %v", diags)
			}
			if !tt.expectError && hasTypeError {
				t.Errorf("unexpected type mismatch error in %v", diags)
			}
		})
	}
}

// TestAnalyzeBlockReferenceResolution verifies that identifiers referencing
// defined blocks are accepted, and union fields accept block references.
func TestAnalyzeBlockReferenceResolution(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			"model ref in union field (str | model)",
			"model gpt4 { provider = \"openai\" }\nagent a { model = gpt4 persona = \"hi\" }",
			false,
		},
		{
			"string in union field",
			"agent a { model = \"gpt-4o\" persona = \"hi\" }",
			false,
		},
		{
			"int in union field rejects",
			"agent a { model = 42 persona = \"hi\" }",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			diags := Analyze(program).Diagnostics

			hasTypeError := false
			for _, d := range diags {
				if d.Severity == diagnostic.Error && strings.Contains(d.Message, "expects type") {
					hasTypeError = true
					break
				}
			}
			if tt.expectError && !hasTypeError {
				t.Errorf("expected type error, got %v", diags)
			}
			if !tt.expectError && hasTypeError {
				t.Errorf("unexpected type error in %v", diags)
			}
		})
	}
}

// TestAnalyzeUndefinedReference verifies that referencing an undefined
// block name produces an error.
func TestAnalyzeUndefinedReference(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorSubstr string
	}{
		{
			"defined reference",
			"model gpt4 { provider = \"openai\" }\nagent a { model = gpt4 persona = \"hi\" }",
			false,
			"",
		},
		{
			"undefined reference",
			"agent a { model = nonexistent persona = \"hi\" }",
			true,
			"undefined",
		},
		{
			"string value not a reference",
			"agent a { model = \"gpt-4o\" persona = \"hi\" }",
			false,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			diags := Analyze(program).Diagnostics
			found := hasErrorContaining(diags, tt.errorSubstr)
			if tt.expectError && !found {
				t.Errorf("expected error containing %q, got %v", tt.errorSubstr, diags)
			}
			if !tt.expectError && found {
				t.Errorf("unexpected error containing %q in %v", tt.errorSubstr, diags)
			}
		})
	}
}

// TestAnalyzeUnknownField verifies that using a field name not in the
// block's schema produces an error.
func TestAnalyzeUnknownField(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			"known field",
			`model gpt4 { provider = "openai" }`,
			false,
		},
		{
			"unknown field",
			`model gpt4 { provider = "openai" foo = "bar" }`,
			true,
		},
		{
			"multiple unknown fields",
			`model gpt4 { provider = "openai" foo = "bar" baz = 42 }`,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			diags := Analyze(program).Diagnostics
			found := hasErrorContaining(diags, "unknown field")
			if tt.expectError && !found {
				t.Errorf("expected unknown field error, got %v", diags)
			}
			if !tt.expectError && found {
				t.Errorf("unexpected unknown field error in %v", diags)
			}
		})
	}
}

// TestAnalyzeDuplicateBlockName verifies that two blocks with the same
// name produce an error.
func TestAnalyzeDuplicateBlockName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			"unique names",
			"model gpt4 { provider = \"openai\" }\nmodel gpt3 { provider = \"openai\" }",
			false,
		},
		{
			"duplicate model names",
			"model gpt4 { provider = \"openai\" }\nmodel gpt4 { provider = \"anthropic\" }",
			true,
		},
		{
			"same name different block types",
			"model gpt4 { provider = \"openai\" }\nagent gpt4 { model = \"x\" persona = \"hi\" }",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			diags := Analyze(program).Diagnostics
			found := hasErrorContaining(diags, "duplicate")
			if tt.expectError && !found {
				t.Errorf("expected duplicate error, got %v", diags)
			}
			if !tt.expectError && found {
				t.Errorf("unexpected duplicate error in %v", diags)
			}
		})
	}
}

// TestAnalyzeDuplicateFieldName verifies that two assignments with the
// same key in a block produce an error.
func TestAnalyzeDuplicateFieldName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			"unique fields",
			`model gpt4 { provider = "openai" temperature = 0.7 }`,
			false,
		},
		{
			"duplicate field",
			`model gpt4 { provider = "openai" provider = "anthropic" }`,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			diags := Analyze(program).Diagnostics
			found := hasErrorContaining(diags, "duplicate field")
			if tt.expectError && !found {
				t.Errorf("expected duplicate field error, got %v", diags)
			}
			if !tt.expectError && found {
				t.Errorf("unexpected duplicate field error in %v", diags)
			}
		})
	}
}

// TestAnalyzeMemberAccess verifies that member access expressions are
// validated: the object must be defined, and the member must exist in
// the object's block schema.
func TestAnalyzeMemberAccess(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorSubstr string
	}{
		{
			"valid member access",
			"model gpt4 { provider = \"openai\" }\nagent a { model = gpt4.provider persona = \"hi\" }",
			false,
			"",
		},
		{
			"undefined object in member access",
			"agent a { model = unknown.provider persona = \"hi\" }",
			true,
			"undefined reference",
		},
		{
			"undefined member on valid object",
			"model gpt4 { provider = \"openai\" }\nagent a { model = gpt4.nonexistent persona = \"hi\" }",
			true,
			"has no field",
		},
		{
			"member access type mismatch",
			"model gpt4 { provider = \"openai\" temperature = 0.7 }\nagent a { model = gpt4.temperature persona = \"hi\" }",
			true,
			"expects type",
		},
		{
			"member access type matches",
			"model gpt4 { provider = \"openai\" }\nagent a { model = gpt4.provider persona = \"hi\" }",
			false,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			diags := Analyze(program).Diagnostics
			found := hasErrorContaining(diags, tt.errorSubstr)
			if tt.expectError && !found {
				t.Errorf("expected error containing %q, got %v", tt.errorSubstr, diags)
			}
			if !tt.expectError && found {
				t.Errorf("unexpected error containing %q in %v", tt.errorSubstr, diags)
			}
		})
	}
}

// TestAnalyzeInputBlock verifies the input block is recognized and validated.
func TestAnalyzeInputBlock(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorSubstr string
	}{
		{
			"valid input with all fields",
			`input apikey {
				type = str
				desc = "the api key"
				default = "sk-xxx"
				sensitive = true
			}`,
			false,
			"",
		},
		{
			"valid input type only",
			`input apikey {
				type = str
			}`,
			false,
			"",
		},
		{
			"input missing required type",
			`input apikey {
				desc = "the api key"
			}`,
			true,
			`missing required field "type"`,
		},
		{
			"input unknown field",
			`input apikey {
				type = str
				bogus = "nope"
			}`,
			true,
			`unknown field "bogus"`,
		},
		{
			"input with schema ref",
			`schema vpc_data_t {
				region = str
			}
			input vpc_data {
				type = vpc_data_t
				desc = "vpc config"
			}`,
			false,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			diags := Analyze(program).Diagnostics
			found := hasErrorContaining(diags, tt.errorSubstr)
			if tt.expectError && !found {
				t.Errorf("expected error containing %q, got %v", tt.errorSubstr, diags)
			}
			if !tt.expectError && found {
				t.Errorf("unexpected error containing %q in %v", tt.errorSubstr, diags)
			}
		})
	}
}

// hasErrorContaining returns true if any error diagnostic contains substr.
func hasErrorContaining(diags []diagnostic.Diagnostic, substr string) bool {
	if substr == "" {
		return false
	}
	for _, d := range diags {
		if d.Severity == diagnostic.Error && strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}


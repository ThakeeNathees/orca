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
			"model with provider and model_name",
			`model gpt4 {
				provider = "openai"
				model_name = "gpt-4o"
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
			`model gpt4 { provider = "openai" model_name = "gpt-4o" }`,
			false,
		},
		{
			"string field with int value",
			`model gpt4 { provider = 42 }`,
			true,
		},
		{
			"float field with float value",
			`model gpt4 { provider = "openai" model_name = "gpt-4o" temperature = 0.7 }`,
			false,
		},
		{
			"float field with string value",
			`model gpt4 { provider = "openai" model_name = "gpt-4o" temperature = "high" }`,
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
			`model gpt4 { provider = "openai" model_name = "gpt-4o" }`,
			false,
		},
		{
			"unknown field",
			`model gpt4 { provider = "openai" model_name = "gpt-4o" foo = "bar" }`,
			true,
		},
		{
			"multiple unknown fields",
			`model gpt4 { provider = "openai" model_name = "gpt-4o" foo = "bar" baz = 42 }`,
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
			`model gpt4 { provider = "openai" model_name = "gpt-4o" temperature = 0.7 }`,
			false,
		},
		{
			"duplicate field",
			`model gpt4 { provider = "openai" model_name = "gpt-4o" provider = "anthropic" }`,
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

// TestInputTypeResolution verifies that input blocks resolve to their
// declared type, so member access works through the schema.
func TestInputTypeResolution(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorSubstr string
	}{
		{
			"access field on input with user schema",
			`schema vpc_data_t {
				region = str
				instance_count = int | null
			}
			input vpc_data {
				type = vpc_data_t
			}
			agent a {
				model = vpc_data.region
				persona = "hi"
			}`,
			false,
			"",
		},
		{
			"access nonexistent field on input",
			`schema vpc_data_t {
				region = str
			}
			input vpc_data {
				type = vpc_data_t
			}
			agent a {
				model = vpc_data.bogus
				persona = "hi"
			}`,
			true,
			`has no field "bogus"`,
		},
		{
			"input with primitive type has no fields",
			`input apikey {
				type = str
			}
			agent a {
				model = apikey.something
				persona = "hi"
			}`,
			true,
			`has no field "something"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			diags := Analyze(program).Diagnostics
			if tt.expectError {
				found := hasErrorContaining(diags, tt.errorSubstr)
				if !found {
					t.Errorf("expected error containing %q, got %v", tt.errorSubstr, diags)
				}
			} else {
				if len(diags) != 0 {
					t.Errorf("expected no diagnostics, got %v", diags)
				}
			}
		})
	}
}

// TestCheckReferencesRecursive verifies that reference checking recurses
// into all expression types (lists, subscriptions, binary expressions, etc.).
func TestCheckReferencesRecursive(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorSubstr string
	}{
		{
			"undefined ref in list element",
			`agent a {
				model = "gpt-4o"
				persona = "hi"
				tools = [nonexistent]
			}`,
			true,
			`undefined reference "nonexistent"`,
		},
		{
			"valid ref in list element",
			`tool web_search {
				name = "web_search"
			}
			agent a {
				model = "gpt-4o"
				persona = "hi"
				tools = [web_search]
			}`,
			false,
			"",
		},
		{
			"undefined ref in subscription object",
			`model gpt5 {
				provider = "openai"
				model_name = nonexistent[0]
			}`,
			true,
			`undefined reference "nonexistent"`,
		},
		{
			"undefined ref in binary expression",
			`workflow w {
				name = nonexistent + "suffix"
			}`,
			true,
			`undefined reference "nonexistent"`,
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

// TestSuppressBlockLevel verifies that @suppress on a block suppresses diagnostics.
func TestSuppressBlockLevel(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorSubstr string
	}{
		{
			"suppress all on block",
			`@suppress
			agent a {}`,
			false,
			"missing required field",
		},
		{
			"suppress specific code on block",
			`@suppress("missing-field")
			agent a {}`,
			false,
			"missing required field",
		},
		{
			"suppress wrong code still reports",
			`@suppress("unknown-field")
			agent a {}`,
			true,
			"missing required field",
		},
		{
			"no suppress reports normally",
			`agent a {}`,
			true,
			"missing required field",
		},
		{
			"suppress duplicate block",
			`model a {
				provider = "openai"
				model_name = "gpt-4o"
			}
			@suppress("duplicate-block")
			model a {
				provider = "openai"
				model_name = "gpt-4o"
			}`,
			false,
			"duplicate block name",
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

// TestSuppressFieldLevel verifies that @suppress on a field suppresses diagnostics.
func TestSuppressFieldLevel(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorSubstr string
	}{
		{
			"suppress undefined ref on field",
			`agent a {
				@suppress("undefined-ref")
				model = nonexistent
				persona = "hi"
			}`,
			false,
			"undefined reference",
		},
		{
			"suppress all on field",
			`agent a {
				@suppress
				model = nonexistent
				persona = "hi"
			}`,
			false,
			"undefined reference",
		},
		{
			"no suppress on field reports",
			`agent a {
				model = nonexistent
				persona = "hi"
			}`,
			true,
			"undefined reference",
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

// TestDiagnosticCodes verifies that diagnostics have the correct code.
func TestDiagnosticCodes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		code  string
	}{
		{
			"undefined ref code",
			`agent a { model = nonexistent persona = "hi" }`,
			diagnostic.CodeUndefinedRef,
		},
		{
			"unknown field code",
			`model m { provider = "openai" model_name = "gpt-4o" bogus = "x" }`,
			diagnostic.CodeUnknownField,
		},
		{
			"missing field code",
			`model m {}`,
			diagnostic.CodeMissingField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			diags := Analyze(program).Diagnostics
			found := false
			for _, d := range diags {
				if d.Code == tt.code {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected diagnostic with code %q, got %v", tt.code, diags)
			}
		})
	}
}

// TestUnifiedTypeSystem verifies that primitives (str, int, etc.) and
// user-defined schemas are treated identically by the type system.
func TestUnifiedTypeSystem(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorSubstr string
	}{
		{
			"string literal matches str field",
			`model m { provider = "openai" model_name = "gpt-4o" }`,
			false,
			"",
		},
		{
			"int literal does not match str field",
			`model m { provider = 42 model_name = "gpt-4o" }`,
			true,
			"expects type str, got int",
		},
		{
			"float literal matches float field",
			`model m { provider = "openai" model_name = "gpt-4o" temperature = 0.7 }`,
			false,
			"",
		},
		{
			"bool literal matches bool field",
			`input x { type = str sensitive = true }`,
			false,
			"",
		},
		{
			"string literal does not match bool field",
			`input x { type = str sensitive = "yes" }`,
			true,
			"expects type bool, got str",
		},
		{
			"null literal in default",
			`input x { type = str default = null }`,
			false,
			"",
		},
		{
			"user schema in input type",
			`schema my_type { name = str }
			input x { type = my_type }`,
			false,
			"",
		},
		{
			"primitive in input type",
			`input x { type = int }`,
			false,
			"",
		},
		{
			"user schema member access works like primitive",
			`schema config { host = str }
			input cfg { type = config }
			workflow w { name = cfg.host }`,
			false,
			"",
		},
		{
			"user schema rejects unknown member",
			`schema config { host = str }
			input cfg { type = config }
			workflow w { name = cfg.nonexistent }`,
			true,
			`has no field "nonexistent"`,
		},
		{
			"inline schema in input type",
			`input cfg {
				type = schema {
					host = str
					port = int
				}
			}
			workflow w { name = cfg.host }`,
			false,
			"",
		},
		{
			"inline schema rejects unknown member",
			`input cfg {
				type = schema {
					host = str
				}
			}
			workflow w { name = cfg.nonexistent }`,
			true,
			`has no field "nonexistent"`,
		},
		{
			"nested inline schema with member access",
			`input user_input {
				type = schema {
					@desc("A list of keys")
					keys = schema {
						openai = str
						google = str
					}

					@desc("A list of models")
					models = schema {
						gpt4o = str
						gemini25 = str
					}
				}
			}
			model gpt5 {
				provider   = user_input.keys.openai
				model_name = user_input.models.gpt4o
			}`,
			false,
			"",
		},
		{
			"nested inline schema rejects unknown nested member",
			`input user_input {
				type = schema {
					keys = schema {
						openai = str
					}
				}
			}
			model gpt5 {
				provider   = user_input.keys.nonexistent
				model_name = "gpt-5"
			}`,
			true,
			`has no field "nonexistent"`,
		},
		{
			"inline schema in agent output",
			`agent writer {
				model   = "gpt4"
				persona = "test"
				output  = schema {
					draft = str
				}
			}`,
			false,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			diags := Analyze(program).Diagnostics
			if tt.expectError {
				found := hasErrorContaining(diags, tt.errorSubstr)
				if !found {
					t.Errorf("expected error containing %q, got %v", tt.errorSubstr, diags)
				}
			} else {
				if len(diags) != 0 {
					t.Errorf("expected no diagnostics, got %v", diags)
				}
			}
		})
	}
}

// TestAnalyzeListSubscriptRequiresInt verifies that subscripting a list with
// a non-integer index produces a diagnostic error.
func TestAnalyzeListSubscriptRequiresInt(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorSubstr string
	}{
		{
			"integer subscript on list literal is valid",
			`model m {
				provider   = "openai"
				model_name = ["a", "b"][0]
			}`,
			false,
			"",
		},
		{
			"string subscript on list literal is invalid",
			`model m {
				provider   = "openai"
				model_name = ["a", "b"]["key"]
			}`,
			true,
			"list subscript requires an integer index, got str",
		},
		{
			"bool subscript on list literal is invalid",
			`model m {
				provider   = "openai"
				model_name = ["a", "b"][true]
			}`,
			true,
			"list subscript requires an integer index, got bool",
		},
		{
			"string subscript on member access list is invalid",
			`tool web_search { name = "web_search" }
			agent a {
				model   = "gpt4"
				persona = "hello"
				tools   = [web_search]
			}
			@suppress("type-mismatch")
			task t {
				agent  = a
				prompt = a.tools["key"]
			}`,
			true,
			"list subscript requires an integer index, got str",
		},
		{
			"string subscript on nested list inside map value is invalid",
			`schema user_defined_thing {
				some_map = map[list[int]]
			}
			input user_input {
				type = user_defined_thing
			}
			model gpt5 {
				provider   = "openai"
				model_name = user_input.some_map[""][""]
			}`,
			true,
			"list subscript requires an integer index, got str",
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
			if !tt.expectError && len(diags) != 0 {
				t.Errorf("expected no diagnostics, got %v", diags)
			}
			if tt.expectError {
				for _, d := range diags {
					if d.Code == diagnostic.CodeInvalidSubscript {
						return
					}
				}
				t.Errorf("expected diagnostic code %q", diagnostic.CodeInvalidSubscript)
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


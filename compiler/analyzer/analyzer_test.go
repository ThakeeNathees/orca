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
	l := lexer.New(input, "")
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
			"agent with all required fields",
			`agent researcher {
				model = "gpt-4o"
				persona = "You are a researcher."
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
			"agent missing persona",
			`agent researcher {
				model = "gpt-4o"
			}`,
			true,
			"persona",
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
			"number field with string value",
			`model gpt4 { provider = "openai" model_name = "gpt-4o" temperature = "high" }`,
			true,
		},
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
			"number field with number value",
			`model gpt4 { provider = "openai" model_name = "gpt-4o" temperature = 0.7 }`,
			false,
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
			"model gpt4 { provider = \"openai\" model_name = \"gpt-4o\" }\nagent a { model = gpt4 persona = \"hi\" }",
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
			"duplicate field",
			`model gpt4 { provider = "openai" model_name = "gpt-4o" provider = "anthropic" }`,
			true,
		},
		{
			"unique fields",
			`model gpt4 { provider = "openai" model_name = "gpt-4o" temperature = 0.7 }`,
			false,
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
				invoke = \(q string) -> q
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
		{
			"undefined ref in ternary condition",
			`model m {
				provider = "openai"
				model_name = nonexistent ? "a" : "b"
			}`,
			true,
			`undefined reference "nonexistent"`,
		},
		{
			"undefined ref in ternary true branch",
			`model m {
				provider = "openai"
				model_name = "x" ? nonexistent : "b"
			}`,
			true,
			`undefined reference "nonexistent"`,
		},
		{
			"undefined ref in ternary false branch",
			`model m {
				provider = "openai"
				model_name = "x" ? "a" : nonexistent
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
			"string literal matches string field",
			`model m { provider = "openai" model_name = "gpt-4o" }`,
			false,
			"",
		},
		{
			"int literal does not match string field",
			`model m { provider = 42 model_name = "gpt-4o" }`,
			true,
			"expects type string, got number",
		},
		{
			"float literal matches float field",
			`model m { provider = "openai" model_name = "gpt-4o" temperature = 0.7 }`,
			false,
			"",
		},
		{
			"bool literal matches bool field",
			`schema flags { flag = bool }
			flags f { flag = true }`,
			false,
			"",
		},
		{
			"string literal does not match bool field",
			`schema flags { flag = bool }
			flags f { flag = "yes" }`,
			true,
			"expects type bool, got string",
		},
		{
			"user schema block instance",
			`schema my_type { name = string }
			my_type x { name = "a" }`,
			false,
			"",
		},
		{
			"user schema member access works like primitive",
			`schema config { host = string }
			config cfg { host = "h" }
			workflow w { name = cfg.host }`,
			false,
			"",
		},
		{
			"user schema rejects unknown member",
			`schema config { host = string }
			config cfg { host = "h" }
			workflow w { name = cfg.nonexistent }`,
			true,
			`has no field "nonexistent"`,
		},
		{
			"nested user schema member access",
			`schema keys_t { openai = string google = string }
			schema models_t { gpt4o = string gemini25 = string }
			schema user_inp { keys = keys_t models = models_t }
			keys_t keys_blk {
				openai = "a"
				google = "b"
			}
			models_t models_blk {
				gpt4o = "x"
				gemini25 = "y"
			}
			user_inp user_input {
				keys = keys_blk
				models = models_blk
			}
			model gpt5 {
				provider   = user_input.keys.openai
				model_name = user_input.models.gpt4o
			}`,
			false,
			"",
		},
		{
			"nested user schema rejects unknown nested member",
			`schema keys_t { openai = string }
			schema user_inp { keys = keys_t }
			keys_t keys_blk {
				openai = "a"
			}
			user_inp user_input {
				keys = keys_blk
			}
			model gpt5 {
				provider   = user_input.keys.nonexistent
				model_name = "gpt-5"
			}`,
			true,
			`has no field "nonexistent"`,
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
			"list subscript requires an integer index, got string",
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
			"string subscript on nested list inside map value is invalid",
			`schema user_defined_thing {
				some_map = map[string, list[int]]
			}
			user_defined_thing user_input {
				some_map = { "": [1, 2] }
			}
			model gpt5 {
				provider   = "openai"
				model_name = user_input.some_map[""][""]
			}`,
			true,
			"list subscript requires an integer index, got string",
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

// --- lambda tests ---

func TestAnalyzeLambdaReturnTypeMismatch(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorSubstr string
	}{
		{
			"recursive lambda matches declared number return",
			`let vars {
				fib = \(n number) number ->
					(n > 1) ? vars.fib(n-1) + vars.fib(n-2) : n
			}`,
			false,
			"",
		},
		{
			"return type mismatch",
			`let v { f = \(n number) string -> n }`,
			true,
			"lambda body type number does not match declared return type string",
		},
		{
			"return type matches",
			`let v { f = \(n number) number -> n }`,
			false,
			"",
		},
		{
			"no return type annotation (no error)",
			`let v { f = \(n number) -> n }`,
			false,
			"",
		},
		{
			"lambda params in scope",
			`let v { f = \(x number) number -> x + 1 }`,
			false,
			"",
		},
		{
			"recursive lambda return type mismatch",
			`let fn {
				fac = \(n number) string -> (n > 1) ? n * fn.fac(n - 1) : 0
			}`,
			true,
			"lambda body type number does not match declared return type string",
		},
		{
			"nested lambda closure captures outer param",
			`let v { f = \(x number) -> \(y number) -> x + y }`,
			false,
			"",
		},
		{
			"nested lambda shadows outer param",
			`let v { f = \(x number) -> \(x string) -> x }`,
			false,
			"",
		},
		{
			"lambda param undefined outside body",
			`let v {
				f = \(x number) -> x
				g = x
			}`,
			true,
			`undefined reference "x"`,
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
		})
	}
}

// --- let block tests ---

func TestAnalyzeLetBlock(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errContains string
	}{
		// TODO: Since 'let' doesnt have a schema let body is not validated (skipped)
		// So the bellow error doesnt arise, fix it.
		// {
		// 	"duplicate field within same let block",
		// 	`let vars {
		// 		x = "first"
		// 		x = "second"
		// 	}`,
		// 	true,
		// 	"duplicate field",
		// },
		{
			"named let block fields accessible via member access",
			`let vars {
				api_url = "https://example.com"
				max_retries = 3
			}
			model gpt4 {
				provider   = "openai"
				model_name = vars.api_url
			}`,
			false,
			"",
		},
		{
			"let variable referenced in agent via member access",
			`let vars { persona_text = "You are helpful." }
			model gpt4 { provider = "openai" model_name = "gpt-4o" }
			agent helper {
				model   = gpt4
				persona = vars.persona_text
			}`,
			false,
			"",
		},
		{
			"duplicate let block name",
			`let vars { x = "a" }
			let vars { y = "b" }`,
			true,
			"duplicate block name",
		},
		{
			"let block name conflicts with other block name",
			`model gpt4 { provider = "openai" model_name = "gpt-4o" }
			let gpt4 { x = "conflict" }`,
			true,
			"duplicate block name",
		},
		{
			"multiple named let blocks",
			`let vars { a = "one" }
			let vars2 { b = "two" }
			model gpt4 {
				provider   = "openai"
				model_name = vars.a
			}`,
			false,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			result := Analyze(program)
			hasErr := len(result.Diagnostics) > 0
			if tt.expectError && !hasErr {
				t.Error("expected diagnostics, got none")
			}
			if !tt.expectError && hasErr {
				t.Errorf("unexpected diagnostics: %v", result.Diagnostics)
			}
			if tt.expectError && tt.errContains != "" {
				found := false
				for _, d := range result.Diagnostics {
					if strings.Contains(d.Message, tt.errContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected diagnostic containing %q, got %v", tt.errContains, result.Diagnostics)
				}
			}
		})
	}
}

// TestAnalyzeWorkflowExpressions verifies that the analyzer validates
// bare expressions in blocks: only workflow blocks allow them, and only
// with the -> operator.
func TestAnalyzeWorkflowExpressions(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errContains string
	}{
		{
			"inline branch missing required route field",
			`workflow wf {
				agent {
					persona = "p"
					model = model {
						provider = "openai"
						model_name = "gpt-4o"
					}
				} -> branch {
				}
			}`,
			true,
			`missing required field "route"`,
		},
		{
			"member access resolves to workflow chain in let block",
			`tool tool_get { invoke = \(inp string) -> "foo" }
			 tool echo_1 { invoke = \(inp string) -> "echo_1: " }
			 tool echo_2 { invoke = \(inp string) -> "echo_2: " }
			 let vars { chain = echo_1 -> echo_2 }
			 workflow wf { tool_get -> vars.chain }`,
			false,
			"",
		},
		{
			"expression in non-workflow block",
			`agent A { model = gpt4 persona = "hi" }
			 @only_assignments
			 model gpt4 { provider = "openai" model_name = "gpt" A }`,
			true,
			"unexpected expression in model block",
		},
		{
			"valid workflow edges",
			`agent A { model = gpt4 }
			 agent B { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 workflow run { A -> B }`,
			false,
			"",
		},
		{
			"valid workflow chain",
			`agent A { model = gpt4 }
			 agent B { model = gpt4 }
			 agent C { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 workflow run { A -> B -> C }`,
			false,
			"",
		},
		{
			"valid workflow with tool node",
			`agent A { model = gpt4 }
			 tool T { invoke = \(x string) -> x }
			 model gpt4 { provider = "openai" }
			 workflow run { A -> T }`,
			false,
			"",
		},
		{
			"valid workflow with cron node",
			`cron daily { schedule = "0 9 * * *" }
			 agent A { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 workflow run { daily -> A }`,
			false,
			"",
		},
		{
			"valid workflow with webhook node",
			`webhook hooks_in { path = "/hooks/in" }
			 agent A { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 workflow run { hooks_in -> A }`,
			false,
			"",
		},
		{
			"model block as workflow node",
			`model gpt4 { provider = "openai" }
			 agent A { model = gpt4 }
			 workflow run { A -> gpt4 }`,
			true,
			"not a valid workflow node",
		},
		{
			"knowledge block as workflow node",
			`knowledge kb { desc = "some docs" }
			 agent A { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 workflow run { A -> kb }`,
			true,
			"not a valid workflow node",
		},
		{
			"non-arrow operator in workflow",
			`agent A { model = gpt4 }
			 agent B { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 workflow run { A + B }`,
			true,
			"only '->' is allowed",
		},
		{
			"string literal in workflow edge",
			`workflow run { "hello" -> "world" }`,
			true,
			"not a valid workflow node",
		},
		{
			"valid inline branch",
			`agent A { model = gpt4 }
			 agent B { model = gpt4 }
			 agent C { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 workflow run { A -> branch { route = { "x": B, "y": C } } }`,
			false,
			"",
		},
		{
			"valid named branch",
			`agent A { model = gpt4 }
			 agent B { model = gpt4 }
			 agent C { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 branch router { route = { "x": B, "y": C } }
			 workflow run { A -> router }`,
			false,
			"",
		},
		{
			"standalone branch in workflow is allowed (dead code)",
			`agent B { model = gpt4 persona = "hi" }
			 model gpt4 { provider = "openai" model_name = "gpt-4o" }
			 workflow run { branch { route = { "x": B } } }`,
			false,
			"",
		},
		{
			"trigger as branch route value is rejected",
			`cron daily { schedule = "0 9 * * *" }
			 agent A { model = gpt4 persona = "hi" }
			 agent B { model = gpt4 persona = "hi" }
			 model gpt4 { provider = "openai" model_name = "gpt-4o" }
			 workflow run { A -> branch { route = { "x": daily, "y": B } } }`,
			true,
			"Triggers can only be workflow entry points",
		},
		{
			"inline trigger as branch route value is rejected",
			`agent A { model = gpt4 persona = "hi" }
			 agent B { model = gpt4 persona = "hi" }
			 model gpt4 { provider = "openai" model_name = "gpt-4o" }
			 workflow run { A -> branch { route = { "x": cron { schedule = "0 9 * * *" }, "y": B } } }`,
			true,
			"Triggers can only be workflow entry points",
		},
		{
			"nested branch routes are recursively validated",
			`cron daily { schedule = "0 9 * * *" }
			 agent A { model = gpt4 persona = "hi" }
			 agent B { model = gpt4 persona = "hi" }
			 model gpt4 { provider = "openai" model_name = "gpt-4o" }
			 workflow run { A -> branch { route = { "x": branch { route = { "y": daily } } } } }`,
			true,
			"Triggers can only be workflow entry points",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			result := Analyze(program)

			if tt.expectError {
				if len(result.Diagnostics) == 0 {
					t.Fatal("expected diagnostics, got none")
				}
				found := false
				for _, d := range result.Diagnostics {
					if strings.Contains(d.Message, tt.errContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected diagnostic containing %q, got %v", tt.errContains, result.Diagnostics)
				}
			} else {
				// Filter to only workflow-related diagnostics.
				for _, d := range result.Diagnostics {
					if d.Code == diagnostic.CodeUnexpectedExpr || d.Code == diagnostic.CodeInvalidWorkNode {
						t.Errorf("unexpected diagnostic: %s", d.Message)
					}
				}
			}
		})
	}
}

// TestAnalyzeTriggerPositions verifies that triggers (cron, webhook) are only
// allowed as the first node in an edge chain and cannot be targets of edges.
func TestAnalyzeTriggerPositions(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errCode     string
		errContains string
	}{
		{
			"trigger as first node is valid",
			`cron daily { schedule = "0 9 * * *" }
			 agent A { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 workflow run { daily -> A }`,
			false,
			"",
			"",
		},
		{
			"trigger as middle node is invalid",
			`cron daily { schedule = "0 9 * * *" }
			 agent A { model = gpt4 }
			 agent B { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 workflow run { A -> daily -> B }`,
			true,
			diagnostic.CodeTriggerAsTarget,
			"Triggers can only be workflow entry points",
		},
		{
			"trigger as last node is invalid",
			`cron daily { schedule = "0 9 * * *" }
			 agent A { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 workflow run { A -> daily }`,
			true,
			diagnostic.CodeTriggerAsTarget,
			"Triggers can only be workflow entry points",
		},
		{
			"trigger-to-trigger chain is invalid",
			`cron daily { schedule = "0 9 * * *" }
			 webhook hooks_in { path = "/hooks/in" }
			 agent A { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 workflow run { daily -> hooks_in -> A }`,
			true,
			diagnostic.CodeTriggerAsTarget,
			"Triggers can only be workflow entry points",
		},
		{
			"webhook as first node is valid",
			`webhook hooks_in { path = "/hooks/in" }
			 agent A { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 workflow run { hooks_in -> A }`,
			false,
			"",
			"",
		},
		{
			"webhook as target of agent is invalid",
			`webhook hooks_in { path = "/hooks/in" }
			 agent A { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 workflow run { A -> hooks_in }`,
			true,
			diagnostic.CodeTriggerAsTarget,
			"Triggers can only be workflow entry points",
		},
		{
			"multiple triggers as first nodes in separate chains is valid",
			`cron daily { schedule = "0 9 * * *" }
			 webhook hooks_in { path = "/hooks/in" }
			 agent A { model = gpt4 }
			 agent B { model = gpt4 }
			 model gpt4 { provider = "openai" }
			 workflow run {
			   daily -> A
			   hooks_in -> B
			 }`,
			false,
			"",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			result := Analyze(program)

			if tt.expectError {
				found := false
				for _, d := range result.Diagnostics {
					if d.Code == tt.errCode && strings.Contains(d.Message, tt.errContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected diagnostic code=%q containing %q, got %v", tt.errCode, tt.errContains, result.Diagnostics)
				}
			} else {
				for _, d := range result.Diagnostics {
					if d.Code == diagnostic.CodeTriggerAsTarget {
						t.Errorf("unexpected trigger diagnostic: %s", d.Message)
					}
				}
			}
		})
	}
}

// TestBlockDependencyOrder verifies that the analyzer produces a topologically
// sorted BlockOrder where dependencies come before dependents.
func TestBlockDependencyOrder(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedOrder []string
	}{
		{
			name: "model before let that references it",
			input: `
				model o { provider = "openai" model_name = "gpt-4o" }
				let vars { m = o }
			`,
			expectedOrder: []string{"o", "vars"},
		},
		{
			name: "model before agent that references it",
			input: `
				agent a1 { model = gpt persona = "test" }
				model gpt { provider = "openai" model_name = "gpt-4o" }
			`,
			expectedOrder: []string{"gpt", "a1"},
		},
		{
			name: "let referencing model, agent referencing let",
			input: `
				agent a1 { model = vars.m persona = "test" }
				let vars { m = o }
				model o { provider = "openai" model_name = "gpt-4o" }
			`,
			expectedOrder: []string{"o", "vars", "a1"},
		},
		{
			name: "independent blocks preserve source order",
			input: `
				model a { provider = "openai" model_name = "a" }
				model b { provider = "openai" model_name = "b" }
			`,
			expectedOrder: []string{"a", "b"},
		},
		{
			name: "workflow depends on agents",
			input: `
				model m { provider = "openai" model_name = "gpt-4o" }
				agent a1 { model = m persona = "p1" }
				agent a2 { model = m persona = "p2" }
				workflow flow { a1 -> a2 }
			`,
			expectedOrder: []string{"m", "a1", "a2", "flow"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			result := Analyze(program)

			for _, d := range result.Diagnostics {
				if d.Severity == diagnostic.Error && d.Code != diagnostic.CodeMissingField {
					t.Fatalf("unexpected error: %s", d.Message)
				}
			}

			if len(result.BlockOrder) != len(tt.expectedOrder) {
				t.Fatalf("BlockOrder length = %d, want %d\ngot:  %v\nwant: %v",
					len(result.BlockOrder), len(tt.expectedOrder), result.BlockOrder, tt.expectedOrder)
			}
			for i, name := range result.BlockOrder {
				if name != tt.expectedOrder[i] {
					t.Errorf("BlockOrder[%d] = %q, want %q\nfull order: %v",
						i, name, tt.expectedOrder[i], result.BlockOrder)
				}
			}
		})
	}
}

// TestBlockDependencyCycleError verifies that cyclic block dependencies
// produce a diagnostic error.
func TestBlockDependencyCycleError(t *testing.T) {
	input := `
		let a { x = b }
		let b { y = a }
	`
	program := parseProgram(t, input)
	result := Analyze(program)

	found := false
	for _, d := range result.Diagnostics {
		if d.Code == diagnostic.CodeCyclicDependency {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected cyclic-dependency diagnostic, got: %v", result.Diagnostics)
	}
}

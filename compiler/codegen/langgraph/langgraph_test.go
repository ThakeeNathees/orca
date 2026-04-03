package langgraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/codegen"
	"github.com/thakee/orca/compiler/codegen/python"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
	"github.com/thakee/orca/compiler/token"
)

// analyzedProgram runs semantic analysis on a parsed AST for tests that construct
// LangGraphBackend with the same shape the compiler uses (analyzer.AnalyzedProgram).
func analyzedProgram(p *ast.Program) analyzer.AnalyzedProgram {
	return analyzer.Analyze(p)
}

// analyzedProgramFromSource parses Orca source and runs semantic analysis (same pipeline
// as tests that load .oc fixtures).
func analyzedProgramFromSource(t *testing.T, source string) analyzer.AnalyzedProgram {
	t.Helper()
	l := lexer.New(source, "")
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	return analyzer.Analyze(program)
}

// testdataProviderConstFoldDir holds .oc inputs for TestGenerateProviderConstFold.
const testdataProviderConstFoldDir = "testdata/provider_const_fold"

// loadProviderConstFoldOC reads a named fixture (without .oc) from testdata/provider_const_fold.
func loadProviderConstFoldOC(t *testing.T, baseName string) string {
	t.Helper()
	path := filepath.Join(testdataProviderConstFoldDir, baseName+".oc")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// findFile searches the output root directory for a file by name.
func findFile(b *LangGraphBackend, name string) string {
	output := b.Generate()
	for _, f := range output.RootDir.Files {
		if f.Name == name {
			return f.Content
		}
	}
	return ""
}

// TestCollectBlocksByKind verifies filtering of block statements by kind.
func TestCollectBlocksByKind(t *testing.T) {
	program := &ast.Program{
		Statements: []ast.Statement{
			&ast.BlockStatement{BaseNode: ast.BaseNode{TokenStart: token.Token{Type: token.MODEL}}, BlockBody: ast.BlockBody{Kind: token.BlockModel}, Name: "m1"},
			&ast.BlockStatement{BaseNode: ast.BaseNode{TokenStart: token.Token{Type: token.AGENT}}, BlockBody: ast.BlockBody{Kind: token.BlockAgent}, Name: "a1"},
			&ast.BlockStatement{BaseNode: ast.BaseNode{TokenStart: token.Token{Type: token.MODEL}}, BlockBody: ast.BlockBody{Kind: token.BlockModel}, Name: "m2"},
			&ast.BlockStatement{BaseNode: ast.BaseNode{TokenStart: token.Token{Type: token.LET}}, BlockBody: ast.BlockBody{Kind: token.BlockLet}},
		},
	}

	tests := []struct {
		name     string
		kind     token.BlockKind
		expected int
	}{
		{"models", token.BlockModel, 2},
		{"agents", token.BlockAgent, 1},
		{"lets", token.BlockLet, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: analyzedProgram(program)}}
			got := b.CollectBlocksByKind(tt.kind)
			if len(got) != tt.expected {
				t.Errorf("expected %d blocks, got %d", tt.expected, len(got))
			}
		})
	}
}

// TestWriteImports verifies that provider imports are written correctly.
func TestWriteImports(t *testing.T) {
	const typedDictImport = "from typing import TypedDict"
	tests := []struct {
		name     string
		imports  []python.PythonImport
		expected []string // substrings after the always-present TypedDict import
	}{
		{
			name:     "no providers",
			imports:  nil,
			expected: nil,
		},
		{
			name:     "openai",
			imports:  []python.PythonImport{providerRegistry["openai"].PyImport},
			expected: []string{"from langchain_openai import ChatOpenAI"},
		},
		{
			name:     "anthropic",
			imports:  []python.PythonImport{providerRegistry["anthropic"].PyImport},
			expected: []string{"from langchain_anthropic import ChatAnthropic"},
		},
		{
			name:     "google",
			imports:  []python.PythonImport{providerRegistry["google"].PyImport},
			expected: []string{"from langchain_google_genai import ChatGoogleGenerativeAI"},
		},
		{
			name: "multiple providers",
			imports: []python.PythonImport{
				providerRegistry["anthropic"].PyImport,
				providerRegistry["openai"].PyImport,
			},
			expected: []string{
				"from langchain_anthropic import ChatAnthropic",
				"from langchain_openai import ChatOpenAI",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: analyzedProgram(&ast.Program{})}}
			var s strings.Builder
			b.writeImports(&s, tt.imports)
			result := s.String()
			if !strings.Contains(result, typedDictImport) {
				t.Errorf("expected %q in output:\n%s", typedDictImport, result)
			}
			for _, exp := range tt.expected {
				if !strings.Contains(result, exp) {
					t.Errorf("expected import %q in output:\n%s", exp, result)
				}
			}
			if len(tt.expected) == 0 && strings.Contains(result, "langchain") {
				t.Errorf("expected no langchain imports, got:\n%s", result)
			}
		})
	}
}

// TestWriteModel verifies Python model instantiation generation.
func TestWriteModel(t *testing.T) {
	tests := []struct {
		name        string
		block       *ast.BlockStatement
		contains    []string
		notContains []string
	}{
		{
			name:  "basic openai model",
			block: modelBlockWithTemp("gpt4", "openai", "gpt-4o", 0.7),
			contains: []string{
				"gpt4 = orca.model(\n",
				`    provider="openai",`,
				`    model_name="gpt-4o",`,
				"    temperature=0.7,",
				")\n",
			},
		},
		{
			name:  "anthropic model",
			block: modelBlockWithTemp("claude", "anthropic", "claude-sonnet-4-20250514", 0.5),
			contains: []string{
				"claude = orca.model(\n",
				`    provider="anthropic",`,
				`    model_name="claude-sonnet-4-20250514",`,
				"    temperature=0.5,",
			},
		},
		{
			name:  "model without temperature",
			block: modelBlock("gemini", "google", "gemini-pro"),
			contains: []string{
				"gemini = orca.model(\n",
				`    provider="google",`,
				`    model_name="gemini-pro",`,
			},
			notContains: []string{
				"temperature=",
			},
		},
		{
			name:  "integer temperature",
			block: modelBlockWithIntTemp("m1", "openai", "gpt-4o", 1),
			contains: []string{
				"    temperature=1,\n",
			},
		},
		{
			name:  "source comment included",
			block: modelBlockAtLine("gpt4", "openai", "gpt-4o", 42),
			contains: []string{
				")\n",
			},
		},
		{
			name:  "google provider",
			block: modelBlock("gem", "google", "gemini-2.0-flash"),
			contains: []string{
				"gem = orca.model(\n",
				`    model_name="gemini-2.0-flash",`,
			},
		},
		{
			name:  "zero temperature",
			block: modelBlockWithIntTemp("m", "openai", "gpt-4o", 0),
			contains: []string{
				"    temperature=0,\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s strings.Builder
			b := &LangGraphBackend{}

			b.writeModel(&s, tt.block)
			result := s.String()

			for _, exp := range tt.contains {
				if !strings.Contains(result, exp) {
					t.Errorf("expected output to contain %q, got:\n%s", exp, result)
				}
			}
			for _, exp := range tt.notContains {
				if strings.Contains(result, exp) {
					t.Errorf("expected output to not contain %q, got:\n%s", exp, result)
				}
			}
		})
	}
}

// TestWriteModelUnknownProvider verifies writeModel produces output even for
// unknown providers — provider validation is handled by resolveProviders.
func TestWriteModelUnknownProvider(t *testing.T) {
	block := modelBlock("m1", "unknown_provider", "some-model")
	var s strings.Builder
	b := &LangGraphBackend{}
	b.writeModel(&s, block)
	if !strings.Contains(s.String(), `provider="unknown_provider"`) {
		t.Fatalf("expected orca.model output, got:\n%s", s.String())
	}
}

// TestWriteAgent verifies Python agent instantiation generation via orca.agent().
func TestWriteAgent(t *testing.T) {
	tests := []struct {
		name     string
		block    *ast.BlockStatement
		contains []string
	}{
		{
			name:  "basic agent without tools",
			block: agentBlock("writer", "gpt4", "You are a helpful writer."),
			contains: []string{
				"writer = orca.agent(\n",
				"    model=gpt4,\n",
				`    persona="You are a helpful writer.",`,
				")\n",
			},
		},
		{
			name:  "agent with tools",
			block: agentBlockWithTools("researcher", "gpt4", "You are a researcher.", []string{"search", "calculator"}),
			contains: []string{
				"researcher = orca.agent(\n",
				"    model=gpt4,\n",
				`    persona="You are a researcher.",`,
				"    tools=[search, calculator],\n",
			},
		},
		{
			name:  "agent with single tool",
			block: agentBlockWithTools("bot", "claude", "You help.", []string{"gmail"}),
			contains: []string{
				"bot = orca.agent(\n",
				"    tools=[gmail],\n",
			},
		},
		{
			name: "source comment included",
			block: &ast.BlockStatement{
				BaseNode: ast.BaseNode{TokenStart: token.Token{Type: token.AGENT, Line: 15}},
				BlockBody: ast.BlockBody{
					Kind: token.BlockAgent,
					Assignments: []*ast.Assignment{
						{Name: "model", Value: &ast.Identifier{Value: "gpt4"}},
						{Name: "persona", Value: &ast.StringLiteral{Value: "test"}},
					},
				},
				Name: "a1",
			},
			contains: []string{
				")\n",
			},
		},
		{
			name:  "persona with escaped quotes",
			block: agentBlock("bot", "gpt4", `You are a "helpful" assistant.`),
			contains: []string{
				`    persona="You are a \"helpful\" assistant.",`,
			},
		},
		{
			name:  "many tools preserves order",
			block: agentBlockWithTools("a", "m", "p", []string{"t1", "t2", "t3", "t4"}),
			contains: []string{
				"    tools=[t1, t2, t3, t4],\n",
			},
		},
		{
			name:  "empty tools list",
			block: agentBlockWithTools("a", "m", "p", nil),
			contains: []string{
				"    tools=[],\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s strings.Builder
			b := &LangGraphBackend{}
			b.writeAgent(&s, tt.block)
			result := s.String()

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("expected output to contain %q, got:\n%s", want, result)
				}
			}
		})
	}
}

// TestProviderDeps verifies pip dependency resolution for providers (via resolveProviders).
func TestProviderDeps(t *testing.T) {
	tests := []struct {
		name             string
		program          *ast.Program
		wantPipAfterCore []string // pip package names after langchain-core
	}{
		{
			name:             "no providers",
			program:          &ast.Program{},
			wantPipAfterCore: nil,
		},
		{
			name: "openai",
			program: programWithModels(
				modelBlock("m1", "openai", "gpt-4o"),
			),
			wantPipAfterCore: []string{"langchain-openai"},
		},
		{
			name: "all providers sorted",
			program: programWithModels(
				modelBlock("m1", "google", "gemini-pro"),
				modelBlock("m2", "openai", "gpt-4o"),
				modelBlock("m3", "anthropic", "claude-sonnet-4-20250514"),
			),
			wantPipAfterCore: []string{"langchain-anthropic", "langchain-google-genai", "langchain-openai"},
		},
		{
			name: "unknown provider ignored",
			program: programWithModels(
				modelBlock("m1", "unknown", "x"),
			),
			wantPipAfterCore: nil,
		},
		{
			name: "duplicates deduplicated",
			program: programWithModels(
				modelBlock("m1", "openai", "gpt-4o"),
				modelBlock("m2", "openai", "gpt-4-turbo"),
			),
			wantPipAfterCore: []string{"langchain-openai"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: analyzedProgram(tt.program)}}
			b.resolveProviders()
			deps := dependenciesFromProviders(b.resolvedProviders)
			var got []string
			for i, d := range deps {
				if i == 0 {
					if d.Name != "langchain-core" {
						t.Fatalf("expected first dependency %q, got %q", "langchain-core", d.Name)
					}
					continue
				}
				got = append(got, d.Name)
			}
			if len(got) != len(tt.wantPipAfterCore) {
				t.Fatalf("expected %d provider pip deps, got %d: %v", len(tt.wantPipAfterCore), len(got), got)
			}
			for i, name := range got {
				if name != tt.wantPipAfterCore[i] {
					t.Errorf("pip dep[%d]: expected %q, got %q", i, tt.wantPipAfterCore[i], name)
				}
			}
		})
	}
}

// TestGenerateMainHeader verifies the auto-generated header is always present.
func TestGenerateMainHeader(t *testing.T) {
	b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: analyzedProgram(&ast.Program{})}}
	mainPy := findFile(b, "main.py")
	if !strings.HasPrefix(mainPy, "# Auto-generated by Orca compiler") {
		t.Errorf("expected auto-generated header, got:\n%s", mainPy)
	}
}

// TestGenerateOutputStructure verifies the tree-based output structure.
func TestGenerateOutputStructure(t *testing.T) {
	b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: analyzedProgram(&ast.Program{})}}
	output := b.Generate()

	if output.RootDir.Name != "build" {
		t.Errorf("expected root dir name %q, got %q", "build", output.RootDir.Name)
	}
	if len(output.RootDir.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(output.RootDir.Files))
	}
	if output.RootDir.Files[0].Name != "orca.py" {
		t.Errorf("expected first file %q, got %q", "orca.py", output.RootDir.Files[0].Name)
	}
	if output.RootDir.Files[1].Name != "main.py" {
		t.Errorf("expected second file %q, got %q", "main.py", output.RootDir.Files[1].Name)
	}
}

// TestCollectDependencies verifies that dependencies are collected from providers.
func TestCollectDependencies(t *testing.T) {
	tests := []struct {
		name     string
		program  *ast.Program
		expected []codegen.Dependency
	}{
		{
			name:    "empty program has langchain-core",
			program: &ast.Program{},
			expected: []codegen.Dependency{
				{Name: "langchain-core"},
			},
		},
		{
			name: "single provider adds its dependency",
			program: programWithModels(
				modelBlock("m1", "openai", "gpt-4o"),
			),
			expected: []codegen.Dependency{
				{Name: "langchain-core"},
				{Name: "langchain-openai"},
			},
		},
		{
			name: "multiple providers sorted",
			program: programWithModels(
				modelBlock("m1", "google", "gemini-pro"),
				modelBlock("m2", "anthropic", "claude-sonnet-4-20250514"),
			),
			expected: []codegen.Dependency{
				{Name: "langchain-core"},
				{Name: "langchain-anthropic"},
				{Name: "langchain-google-genai"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: analyzedProgram(tt.program)}}
			output := b.Generate()
			if len(output.Dependencies) != len(tt.expected) {
				t.Fatalf("expected %d deps, got %d: %v", len(tt.expected), len(output.Dependencies), output.Dependencies)
			}
			for i, dep := range output.Dependencies {
				if dep.Name != tt.expected[i].Name {
					t.Errorf("dep[%d]: expected name %q, got %q", i, tt.expected[i].Name, dep.Name)
				}
			}
		})
	}
}

// TestGenerateProviderConstFold verifies resolveProviders uses analyzer.ConstFold on
// the model provider field. Each case is a real .oc file under testdata/provider_const_fold.
func TestGenerateProviderConstFold(t *testing.T) {
	tests := []struct {
		fixture        string // base name of testdata/provider_const_fold/<fixture>.oc
		wantDeps       []string
		wantImport     string // substring of main.py; empty to skip
		wantDiagCount  int
		wantDiagCode   string
		wantDiagSubstr string
	}{
		{
			fixture:    "concat_openai",
			wantDeps:   []string{"langchain-core", "langchain-openai"},
			wantImport: "from langchain_openai import ChatOpenAI",
		},
		{
			fixture:    "nested_concat_anthropic",
			wantDeps:   []string{"langchain-core", "langchain-anthropic"},
			wantImport: "from langchain_anthropic import ChatAnthropic",
		},
		{
			fixture:    "member_access_let",
			wantDeps:   []string{"langchain-core", "langchain-openai"},
			wantImport: "from langchain_openai import ChatOpenAI",
		},
		{
			fixture:        "folded_unknown_provider",
			wantDeps:       []string{"langchain-core"},
			wantDiagCount:  1,
			wantDiagCode:   diagnostic.CodeUnknownProvider,
			wantDiagSubstr: `unknown provider "bad_provider"`,
		},
		{
			fixture:        "non_string_provider",
			wantDeps:       []string{"langchain-core"},
			wantDiagCount:  1,
			wantDiagCode:   diagnostic.CodeTypeMismatch,
			wantDiagSubstr: `field "provider" expects type str, got int`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			src := loadProviderConstFoldOC(t, tt.fixture)
			ap := analyzedProgramFromSource(t, src)
			// non_string_provider.oc intentionally violates the model schema (int for provider);
			// the analyzer reports that before codegen reports a non-string constant fold.
			if tt.fixture != "non_string_provider" && len(ap.Diagnostics) > 0 {
				t.Fatalf("unexpected analyzer diagnostics before codegen: %v", ap.Diagnostics)
			}
			b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: ap}}
			out := b.Generate()

			if len(out.Dependencies) != len(tt.wantDeps) {
				t.Fatalf("dependencies: got %d (%v), want %d (%v)",
					len(out.Dependencies), depNames(out.Dependencies), len(tt.wantDeps), tt.wantDeps)
			}
			for i := range tt.wantDeps {
				if out.Dependencies[i].Name != tt.wantDeps[i] {
					t.Errorf("dependencies[%d]: got %q, want %q", i, out.Dependencies[i].Name, tt.wantDeps[i])
				}
			}

			if tt.wantDiagCount != len(out.Diagnostics) {
				t.Fatalf("diagnostics: got %d (%v), want %d", len(out.Diagnostics), out.Diagnostics, tt.wantDiagCount)
			}
			if tt.wantDiagCount > 0 {
				d := out.Diagnostics[0]
				if d.Code != tt.wantDiagCode {
					t.Errorf("diagnostic code: got %q, want %q", d.Code, tt.wantDiagCode)
				}
				if tt.wantDiagSubstr != "" && !strings.Contains(d.Message, tt.wantDiagSubstr) {
					t.Errorf("diagnostic message: got %q, want substring %q", d.Message, tt.wantDiagSubstr)
				}
			}

			if tt.wantImport != "" {
				mainPy := findFile(b, "main.py")
				if !strings.Contains(mainPy, tt.wantImport) {
					t.Errorf("main.py should contain %q, got:\n%s", tt.wantImport, mainPy)
				}
			}
		})
	}
}

func depNames(deps []codegen.Dependency) []string {
	out := make([]string, len(deps))
	for i, d := range deps {
		out[i] = d.Name
	}
	return out
}

// TestValidateProviders verifies that unknown providers produce error diagnostics.
func TestValidateProviders(t *testing.T) {
	tests := []struct {
		name        string
		program     *ast.Program
		expectDiags int
		expectMsg   string
	}{
		{
			name:        "known provider no diagnostics",
			program:     programWithModels(modelBlock("m1", "openai", "gpt-4o")),
			expectDiags: 0,
		},
		{
			name:        "unknown provider produces error",
			program:     programWithModels(modelBlock("m1", "unknown_provider", "some-model")),
			expectDiags: 1,
			expectMsg:   `unknown provider "unknown_provider"`,
		},
		{
			name: "mixed known and unknown",
			program: programWithModels(
				modelBlock("m1", "openai", "gpt-4o"),
				modelBlock("m2", "bad_provider", "x"),
			),
			expectDiags: 1,
			expectMsg:   `unknown provider "bad_provider"`,
		},
		{
			name: "multiple unknown providers",
			program: programWithModels(
				modelBlock("m1", "foo", "x"),
				modelBlock("m2", "bar", "y"),
			),
			expectDiags: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: analyzedProgram(tt.program)}}
			output := b.Generate()
			if len(output.Diagnostics) != tt.expectDiags {
				t.Fatalf("expected %d diagnostics, got %d: %v", tt.expectDiags, len(output.Diagnostics), output.Diagnostics)
			}
			for _, d := range output.Diagnostics {
				if d.Severity != diagnostic.Error {
					t.Errorf("expected error severity, got %v", d.Severity)
				}
				if d.Code != diagnostic.CodeUnknownProvider {
					t.Errorf("expected code %q, got %q", diagnostic.CodeUnknownProvider, d.Code)
				}
			}
			if tt.expectMsg != "" && tt.expectDiags > 0 {
				if !strings.Contains(output.Diagnostics[0].Message, tt.expectMsg) {
					t.Errorf("expected message containing %q, got %q", tt.expectMsg, output.Diagnostics[0].Message)
				}
			}
		})
	}
}

// --- Test helpers ---

// modelBlock creates a model block with provider and model_name fields.
func modelBlock(name, provider, modelName string) *ast.BlockStatement {
	return &ast.BlockStatement{
		BaseNode: ast.BaseNode{TokenStart: token.Token{Type: token.MODEL, Literal: "model"}},
		BlockBody: ast.BlockBody{
			Kind: token.BlockModel,
			Assignments: []*ast.Assignment{
				{Name: "provider", Value: &ast.StringLiteral{Value: provider}},
				{Name: "model_name", Value: &ast.StringLiteral{Value: modelName}},
			},
		},
		Name: name,
	}
}

// modelBlockWithTemp creates a model block with a float temperature.
func modelBlockWithTemp(name, provider, modelName string, temp float64) *ast.BlockStatement {
	block := modelBlock(name, provider, modelName)
	block.Assignments = append(block.Assignments, &ast.Assignment{
		Name:  "temperature",
		Value: &ast.FloatLiteral{Value: temp},
	})
	return block
}

// modelBlockWithIntTemp creates a model block with an integer temperature.
func modelBlockWithIntTemp(name, provider, modelName string, temp int64) *ast.BlockStatement {
	block := modelBlock(name, provider, modelName)
	block.Assignments = append(block.Assignments, &ast.Assignment{
		Name:  "temperature",
		Value: &ast.IntegerLiteral{Value: temp},
	})
	return block
}

// modelBlockAtLine creates a model block at a specific source line.
func modelBlockAtLine(name, provider, modelName string, line int) *ast.BlockStatement {
	block := modelBlock(name, provider, modelName)
	block.BaseNode = ast.BaseNode{TokenStart: token.Token{Type: token.MODEL, Literal: "model", Line: line}}
	return block
}

// agentBlock creates an agent block with model and persona.
func agentBlock(name, model, persona string) *ast.BlockStatement {
	return &ast.BlockStatement{
		BaseNode: ast.BaseNode{TokenStart: token.Token{Type: token.AGENT, Literal: "agent"}},
		BlockBody: ast.BlockBody{
			Kind: token.BlockAgent,
			Assignments: []*ast.Assignment{
				{Name: "model", Value: &ast.Identifier{Value: model}},
				{Name: "persona", Value: &ast.StringLiteral{Value: persona}},
			},
		},
		Name: name,
	}
}

// agentBlockWithTools creates an agent block with model, persona, and tools.
func agentBlockWithTools(name, model, persona string, tools []string) *ast.BlockStatement {
	block := agentBlock(name, model, persona)
	elems := make([]ast.Expression, len(tools))
	for i, t := range tools {
		elems[i] = &ast.Identifier{Value: t}
	}
	block.Assignments = append(block.Assignments, &ast.Assignment{
		Name:  "tools",
		Value: &ast.ListLiteral{Elements: elems},
	})
	return block
}

// programWithModels creates a program with model block statements.
func programWithModels(blocks ...*ast.BlockStatement) *ast.Program {
	stmts := make([]ast.Statement, len(blocks))
	for i, b := range blocks {
		stmts[i] = b
	}
	return &ast.Program{Statements: stmts}
}

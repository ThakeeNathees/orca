package langgraph

import (
	"strings"
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/codegen"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/token"
)

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

// TestCollectProviders verifies that collectProviders extracts and deduplicates
// provider names from model blocks.
func TestCollectProviders(t *testing.T) {
	tests := []struct {
		name     string
		program  *ast.Program
		expected []string
	}{
		{
			name:     "no models",
			program:  &ast.Program{},
			expected: nil,
		},
		{
			name: "single provider",
			program: programWithModels(
				modelBlock("m1", "openai", "gpt-4o"),
			),
			expected: []string{"openai"},
		},
		{
			name: "multiple unique providers",
			program: programWithModels(
				modelBlock("m1", "openai", "gpt-4o"),
				modelBlock("m2", "anthropic", "claude-sonnet-4-20250514"),
			),
			expected: []string{"anthropic", "openai"},
		},
		{
			name: "duplicate providers deduplicated",
			program: programWithModels(
				modelBlock("m1", "openai", "gpt-4o"),
				modelBlock("m2", "openai", "gpt-4-turbo"),
			),
			expected: []string{"openai"},
		},
		{
			name: "all three providers sorted",
			program: programWithModels(
				modelBlock("m1", "google", "gemini-pro"),
				modelBlock("m2", "openai", "gpt-4o"),
				modelBlock("m3", "anthropic", "claude-sonnet-4-20250514"),
			),
			expected: []string{"anthropic", "google", "openai"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: tt.program}}
			got := b.CollectProviders()
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d providers, got %d: %v", len(tt.expected), len(got), got)
			}
			for i, p := range got {
				if p != tt.expected[i] {
					t.Errorf("provider[%d]: expected %q, got %q", i, tt.expected[i], p)
				}
			}
		})
	}
}

// TestCollectBlocks verifies filtering of block statements by token type.
func TestCollectBlocks(t *testing.T) {
	program := &ast.Program{
		Statements: []ast.Statement{
			&ast.BlockStatement{BaseNode: ast.BaseNode{TokenStart: token.Token{Type: token.MODEL}}, Name: "m1"},
			&ast.BlockStatement{BaseNode: ast.BaseNode{TokenStart: token.Token{Type: token.AGENT}}, Name: "a1"},
			&ast.BlockStatement{BaseNode: ast.BaseNode{TokenStart: token.Token{Type: token.MODEL}}, Name: "m2"},
			&ast.BlockStatement{BaseNode: ast.BaseNode{TokenStart: token.Token{Type: token.LET}}},
		},
	}

	tests := []struct {
		name      string
		tokenType token.TokenType
		expected  int
	}{
		{"models", token.MODEL, 2},
		{"agents", token.AGENT, 1},
		{"lets", token.LET, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: program}}
			got := b.CollectBlocks(tt.tokenType)
			if len(got) != tt.expected {
				t.Errorf("expected %d blocks, got %d", tt.expected, len(got))
			}
		})
	}
}

// TestWriteImports verifies that provider imports are written correctly.
func TestWriteImports(t *testing.T) {
	tests := []struct {
		name      string
		providers []string
		expected  []string
	}{
		{
			name:      "no providers",
			providers: nil,
			expected:  nil,
		},
		{
			name:      "openai",
			providers: []string{"openai"},
			expected:  []string{"from langchain_openai import ChatOpenAI"},
		},
		{
			name:      "anthropic",
			providers: []string{"anthropic"},
			expected:  []string{"from langchain_anthropic import ChatAnthropic"},
		},
		{
			name:      "google",
			providers: []string{"google"},
			expected:  []string{"from langchain_google_genai import ChatGoogleGenerativeAI"},
		},
		{
			name:      "unknown provider skipped",
			providers: []string{"unknown"},
			expected:  nil,
		},
		{
			name:      "multiple providers",
			providers: []string{"anthropic", "openai"},
			expected: []string{
				"from langchain_anthropic import ChatAnthropic",
				"from langchain_openai import ChatOpenAI",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: &ast.Program{}}}
			var s strings.Builder
			b.writeImports(&s, tt.providers)
			result := s.String()
			for _, exp := range tt.expected {
				if !strings.Contains(result, exp) {
					t.Errorf("expected import %q in output:\n%s", exp, result)
				}
			}
			if len(tt.expected) == 0 && result != "" {
				t.Errorf("expected empty output, got:\n%s", result)
			}
		})
	}
}

// TestWriteModel verifies Python model instantiation generation.
func TestWriteModel(t *testing.T) {
	tests := []struct {
		name     string
		block    *ast.BlockStatement
		contains []string
		empty    bool
	}{
		{
			name:  "basic openai model",
			block: modelBlockWithTemp("gpt4", "openai", "gpt-4o", 0.7),
			contains: []string{
				"gpt4 = ChatOpenAI(",
				`    model="gpt-4o",`,
				"    temperature=0.7,",
			},
		},
		{
			name:  "anthropic model",
			block: modelBlockWithTemp("claude", "anthropic", "claude-sonnet-4-20250514", 0.5),
			contains: []string{
				"claude = ChatAnthropic(",
				`    model="claude-sonnet-4-20250514",`,
				"    temperature=0.5,",
			},
		},
		{
			name:  "model without temperature",
			block: modelBlock("gemini", "google", "gemini-pro"),
			contains: []string{
				"gemini = ChatGoogleGenerativeAI(",
				`    model="gemini-pro",`,
			},
		},
		{
			name:  "integer temperature",
			block: modelBlockWithIntTemp("m1", "openai", "gpt-4o", 1),
			contains: []string{
				"    temperature=1.0,",
			},
		},
		{
			name:  "unknown provider produces nothing",
			block: modelBlock("m1", "unknown_provider", "some-model"),
			empty: true,
		},
		{
			name:  "source comment included",
			block: modelBlockAtLine("gpt4", "openai", "gpt-4o", 42),
			contains: []string{
				"# line 42",
			},
		},
		{
			name:  "closing paren on its own line",
			block: modelBlock("m", "openai", "gpt-4o"),
			contains: []string{
				"\n)",
			},
		},
		{
			name:  "google provider class",
			block: modelBlock("gem", "google", "gemini-2.0-flash"),
			contains: []string{
				"gem = ChatGoogleGenerativeAI(",
				`    model="gemini-2.0-flash",`,
			},
		},
		{
			name:  "zero temperature",
			block: modelBlockWithIntTemp("m", "openai", "gpt-4o", 0),
			contains: []string{
				"    temperature=0.0,",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s strings.Builder
			writeModel(&s, tt.block)
			result := s.String()

			if tt.empty {
				if result != "" {
					t.Errorf("expected empty output, got:\n%s", result)
				}
				return
			}

			for _, exp := range tt.contains {
				if !strings.Contains(result, exp) {
					t.Errorf("expected %q in output:\n%s", exp, result)
				}
			}
		})
	}
}

// TestWriteAgent verifies Python agent generation from agent blocks.
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
				"writer = create_react_agent(",
				"    gpt4,",
				`    prompt="You are a helpful writer.",`,
			},
		},
		{
			name:  "agent with tools",
			block: agentBlockWithTools("researcher", "gpt4", "You are a researcher.", []string{"search", "calculator"}),
			contains: []string{
				"researcher = create_react_agent(",
				"    gpt4,",
				"    tools=[search, calculator],",
				`    prompt="You are a researcher.",`,
			},
		},
		{
			name:  "agent with single tool",
			block: agentBlockWithTools("bot", "claude", "You help.", []string{"gmail"}),
			contains: []string{
				"bot = create_react_agent(",
				"    claude,",
				"    tools=[gmail],",
				`    prompt="You help.",`,
			},
		},
		{
			name: "source comment included",
			block: &ast.BlockStatement{
				BaseNode: ast.BaseNode{TokenStart: token.Token{Type: token.AGENT, Line: 15}},
				Name:     "a1",
				Assignments: []*ast.Assignment{
					{Name: "model", Value: &ast.Identifier{Value: "gpt4"}},
					{Name: "persona", Value: &ast.StringLiteral{Value: "test"}},
				},
			},
			contains: []string{
				"# line 15",
			},
		},
		{
			name:  "closing paren on its own line",
			block: agentBlock("a", "m", "p"),
			contains: []string{
				"\n)",
			},
		},
		{
			name:  "persona with escaped quotes",
			block: agentBlock("bot", "gpt4", `You are a "helpful" assistant.`),
			contains: []string{
				`prompt="You are a \"helpful\" assistant."`,
			},
		},
		{
			name:  "many tools preserves order",
			block: agentBlockWithTools("a", "m", "p", []string{"t1", "t2", "t3", "t4"}),
			contains: []string{
				"tools=[t1, t2, t3, t4],",
			},
		},
		{
			name:  "empty tools list omitted",
			block: agentBlockWithTools("a", "m", "p", nil),
			contains: []string{
				"    m,\n    prompt=",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s strings.Builder
			writeAgent(&s, tt.block)
			result := s.String()

			for _, exp := range tt.contains {
				if !strings.Contains(result, exp) {
					t.Errorf("expected %q in output:\n%s", exp, result)
				}
			}
		})
	}
}

// TestProviderDeps verifies pip dependency resolution for providers.
func TestProviderDeps(t *testing.T) {
	tests := []struct {
		name      string
		providers []string
		expected  []string
	}{
		{
			name:      "no providers",
			providers: nil,
			expected:  nil,
		},
		{
			name:      "openai",
			providers: []string{"openai"},
			expected:  []string{"langchain-openai"},
		},
		{
			name:      "all providers sorted",
			providers: []string{"google", "openai", "anthropic"},
			expected:  []string{"langchain-anthropic", "langchain-google-genai", "langchain-openai"},
		},
		{
			name:      "unknown provider ignored",
			providers: []string{"unknown"},
			expected:  nil,
		},
		{
			name:      "duplicates deduplicated",
			providers: []string{"openai", "openai"},
			expected:  []string{"langchain-openai"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := providerDeps(tt.providers)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d deps, got %d: %v", len(tt.expected), len(got), got)
			}
			for i, d := range got {
				if d != tt.expected[i] {
					t.Errorf("dep[%d]: expected %q, got %q", i, tt.expected[i], d)
				}
			}
		})
	}
}

// TestGenerateMainHeader verifies the auto-generated header is always present.
func TestGenerateMainHeader(t *testing.T) {
	b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: &ast.Program{}}}
	mainPy := findFile(b, "main.py")
	if !strings.HasPrefix(mainPy, "# Auto-generated by Orca compiler") {
		t.Errorf("expected auto-generated header, got:\n%s", mainPy)
	}
}

// TestGenerateOutputStructure verifies the tree-based output structure.
func TestGenerateOutputStructure(t *testing.T) {
	b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: &ast.Program{}}}
	output := b.Generate()

	if output.RootDir.Name != "build" {
		t.Errorf("expected root dir name %q, got %q", "build", output.RootDir.Name)
	}
	if len(output.RootDir.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(output.RootDir.Files))
	}
	if output.RootDir.Files[0].Name != "main.py" {
		t.Errorf("expected first file %q, got %q", "main.py", output.RootDir.Files[0].Name)
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
			b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: tt.program}}
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
			b := &LangGraphBackend{BaseBackend: codegen.BaseBackend{Program: tt.program}}
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
		Name:     name,
		Assignments: []*ast.Assignment{
			{Name: "provider", Value: &ast.StringLiteral{Value: provider}},
			{Name: "model_name", Value: &ast.StringLiteral{Value: modelName}},
		},
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
		Name:     name,
		Assignments: []*ast.Assignment{
			{Name: "model", Value: &ast.Identifier{Value: model}},
			{Name: "persona", Value: &ast.StringLiteral{Value: persona}},
		},
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

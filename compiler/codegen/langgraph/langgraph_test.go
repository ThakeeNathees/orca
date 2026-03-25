package langgraph

import (
	"strings"
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

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
			b := New(tt.program)
			got := b.collectProviders()
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
		{"tasks (none)", token.TASK, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(program)
			got := b.collectBlocks(tt.tokenType)
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
			b := New(&ast.Program{})
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
				`model="gpt-4o"`,
				"temperature=0.7",
			},
		},
		{
			name:  "anthropic model",
			block: modelBlockWithTemp("claude", "anthropic", "claude-sonnet-4-20250514", 0.5),
			contains: []string{
				"claude = ChatAnthropic(",
				`model="claude-sonnet-4-20250514"`,
				"temperature=0.5",
			},
		},
		{
			name:  "model without temperature",
			block: modelBlock("gemini", "google", "gemini-pro"),
			contains: []string{
				"gemini = ChatGoogleGenerativeAI(",
				`model="gemini-pro"`,
			},
		},
		{
			name:  "integer temperature",
			block: modelBlockWithIntTemp("m1", "openai", "gpt-4o", 1),
			contains: []string{
				"temperature=1.0",
			},
		},
		{
			name: "unknown provider produces nothing",
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
	b := New(&ast.Program{})
	output := b.Generate()
	if !strings.HasPrefix(output.MainPy, "# Auto-generated by Orca compiler") {
		t.Errorf("expected auto-generated header, got:\n%s", output.MainPy)
	}
}

// TestGeneratePyProjectStructure verifies the pyproject.toml structure.
func TestGeneratePyProjectStructure(t *testing.T) {
	tests := []struct {
		name     string
		program  *ast.Program
		contains []string
	}{
		{
			name:    "empty program",
			program: &ast.Program{},
			contains: []string{
				"[project]",
				`name = "orca-build"`,
				`version = "0.1.0"`,
				`requires-python = ">=3.11"`,
				`"langchain-core"`,
			},
		},
		{
			name: "with openai provider",
			program: programWithModels(
				modelBlock("m1", "openai", "gpt-4o"),
			),
			contains: []string{
				`"langchain-core"`,
				`"langchain-openai"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.program)
			output := b.Generate()
			for _, exp := range tt.contains {
				if !strings.Contains(output.PyProjectTOML, exp) {
					t.Errorf("expected %q in toml:\n%s", exp, output.PyProjectTOML)
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

// programWithModels creates a program with model block statements.
func programWithModels(blocks ...*ast.BlockStatement) *ast.Program {
	stmts := make([]ast.Statement, len(blocks))
	for i, b := range blocks {
		stmts[i] = b
	}
	return &ast.Program{Statements: stmts}
}

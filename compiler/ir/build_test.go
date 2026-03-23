package ir

import (
	"testing"

	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
)

// buildIR is a test helper that parses, analyzes, and builds the IR.
func buildIR(t *testing.T, input string) *IR {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	result := analyzer.Analyze(program)
	if len(result.Diagnostics) > 0 {
		t.Fatalf("analyzer diagnostics: %v", result.Diagnostics)
	}
	return Build(program)
}

// TestBuildEmptyProgram verifies that an empty program produces an empty IR.
func TestBuildEmptyProgram(t *testing.T) {
	ir := buildIR(t, "")
	if len(ir.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(ir.Models))
	}
	if len(ir.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(ir.Agents))
	}
}

// TestBuildModel verifies that model blocks are resolved into ModelDefs.
func TestBuildModel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		model    string
		provider string
		modelName string
		hasTemp  bool
		temp     float64
	}{
		{
			"basic model",
			`model gpt4 {
				provider    = "openai"
				model_name  = "gpt-4o"
				temperature = 0.7
			}`,
			"gpt4", "openai", "gpt-4o", true, 0.7,
		},
		{
			"model without temperature",
			`model claude {
				provider   = "anthropic"
				model_name = "claude-3-opus"
			}`,
			"claude", "anthropic", "claude-3-opus", false, 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ir := buildIR(t, tt.input)
			if len(ir.Models) != 1 {
				t.Fatalf("expected 1 model, got %d", len(ir.Models))
			}
			m := ir.Models[0]
			if m.Name != tt.model {
				t.Errorf("Name = %q, want %q", m.Name, tt.model)
			}
			if m.Provider != tt.provider {
				t.Errorf("Provider = %q, want %q", m.Provider, tt.provider)
			}
			if m.ModelName != tt.modelName {
				t.Errorf("ModelName = %q, want %q", m.ModelName, tt.modelName)
			}
			if m.HasTemp != tt.hasTemp {
				t.Errorf("HasTemp = %v, want %v", m.HasTemp, tt.hasTemp)
			}
			if m.HasTemp && m.Temperature != tt.temp {
				t.Errorf("Temperature = %v, want %v", m.Temperature, tt.temp)
			}
		})
	}
}

// TestBuildProviders verifies that unique providers are collected.
func TestBuildProviders(t *testing.T) {
	input := `
		model gpt4 {
			provider   = "openai"
			model_name = "gpt-4o"
		}
		model gpt5 {
			provider   = "openai"
			model_name = "gpt-5"
		}
		model claude {
			provider   = "anthropic"
			model_name = "claude-3-opus"
		}
	`
	ir := buildIR(t, input)
	if len(ir.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d: %v", len(ir.Providers), ir.Providers)
	}
}

// TestBuildAgent verifies that agent blocks are resolved into AgentDefs.
func TestBuildAgent(t *testing.T) {
	input := `
		model gpt4 {
			provider   = "openai"
			model_name = "gpt-4o"
		}
		tool web_search {
			name = "web_search"
		}
		agent researcher {
			model   = gpt4
			persona = "Research the topic"
			tools   = [web_search]
		}
	`
	ir := buildIR(t, input)
	if len(ir.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(ir.Agents))
	}
	a := ir.Agents[0]
	if a.Name != "researcher" {
		t.Errorf("Name = %q, want %q", a.Name, "researcher")
	}
	if a.Model != "gpt4" {
		t.Errorf("Model = %q, want %q", a.Model, "gpt4")
	}
	if a.Persona != "Research the topic" {
		t.Errorf("Persona = %q, want %q", a.Persona, "Research the topic")
	}
	if len(a.Tools) != 1 || a.Tools[0] != "web_search" {
		t.Errorf("Tools = %v, want [web_search]", a.Tools)
	}
}

// TestBuildLet verifies that let blocks are resolved into LetVars.
func TestBuildLet(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string // expected variable names
	}{
		{
			"single let block",
			`let {
				api_url = "https://example.com"
				retries = 3
			}`,
			[]string{"api_url", "retries"},
		},
		{
			"multiple let blocks merged",
			`let { a = "one" }
			let { b = "two" }`,
			[]string{"a", "b"},
		},
		{
			"empty let block",
			`let {}`,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ir := buildIR(t, tt.input)
			if len(ir.Lets) != len(tt.expected) {
				t.Fatalf("expected %d lets, got %d", len(tt.expected), len(ir.Lets))
			}
			for i, name := range tt.expected {
				if ir.Lets[i].Name != name {
					t.Errorf("Lets[%d].Name = %q, want %q", i, ir.Lets[i].Name, name)
				}
				if ir.Lets[i].Value == nil {
					t.Errorf("Lets[%d].Value is nil", i)
				}
			}
		})
	}
}

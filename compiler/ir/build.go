package ir

import (
	"sort"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

// Build walks the analyzed AST and produces a fully resolved IR.
// All references (model names, tool names) are resolved to strings.
func Build(program *ast.Program) *IR {
	b := &builder{
		providers: make(map[string]bool),
	}
	return b.build(program)
}

type builder struct {
	providers map[string]bool
}

func (b *builder) build(program *ast.Program) *IR {
	result := &IR{}

	for _, stmt := range program.Statements {
		block, ok := stmt.(*ast.BlockStatement)
		if !ok {
			continue
		}
		switch block.TokenStart.Type {
		case token.LET:
			for _, assign := range block.Assignments {
				result.Lets = append(result.Lets, LetVar{
					Name:   assign.Name,
					Value:  assign.Value,
					File:   block.SourceFile,
					Line:   assign.TokenStart.Line,
					Column: assign.TokenStart.Column,
				})
			}
		case token.MODEL:
			result.Models = append(result.Models, b.buildModel(block))
		case token.AGENT:
			result.Agents = append(result.Agents, b.buildAgent(block))
		}
	}

	// Sorted for deterministic output.
	for p := range b.providers {
		result.Providers = append(result.Providers, p)
	}
	sort.Strings(result.Providers)

	return result
}

func (b *builder) buildModel(block *ast.BlockStatement) ModelDef {
	m := ModelDef{
		Name:   block.Name,
		File:   block.SourceFile,
		Line:   block.TokenStart.Line,
		Column: block.TokenStart.Column,
	}

	for _, assign := range block.Assignments {
		switch assign.Name {
		case "provider":
			m.Provider = stringValue(assign.Value)
			b.providers[m.Provider] = true
		case "model_name":
			m.ModelName = stringValue(assign.Value)
		case "temperature":
			m.Temperature = floatValue(assign.Value)
			m.HasTemp = true
		}
	}

	return m
}

func (b *builder) buildAgent(block *ast.BlockStatement) AgentDef {
	a := AgentDef{
		Name:   block.Name,
		File:   block.SourceFile,
		Line:   block.TokenStart.Line,
		Column: block.TokenStart.Column,
	}

	for _, assign := range block.Assignments {
		switch assign.Name {
		case "model":
			a.Model = identOrStringValue(assign.Value)
		case "persona":
			a.Persona = stringValue(assign.Value)
		case "tools":
			if list, ok := assign.Value.(*ast.ListLiteral); ok {
				for _, elem := range list.Elements {
					a.Tools = append(a.Tools, identOrStringValue(elem))
				}
			}
		}
	}

	return a
}

func stringValue(expr ast.Expression) string {
	if s, ok := expr.(*ast.StringLiteral); ok {
		return s.Value
	}
	return ""
}

func floatValue(expr ast.Expression) float64 {
	switch e := expr.(type) {
	case *ast.FloatLiteral:
		return e.Value
	case *ast.IntegerLiteral:
		return float64(e.Value)
	}
	return 0
}

func identOrStringValue(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Value
	case *ast.StringLiteral:
		return e.Value
	}
	return ""
}

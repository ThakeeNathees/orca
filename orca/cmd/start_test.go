package cmd

import (
	"strings"
	"testing"

	"github.com/thakee/orca/orca/compiler/ast"
	"github.com/thakee/orca/orca/compiler/lexer"
	"github.com/thakee/orca/orca/compiler/parser"
	"github.com/thakee/orca/orca/compiler/types"
)

func parseProgramStartTest(t *testing.T, src string) *ast.Program {
	t.Helper()
	l := lexer.New(src, "start_test.orca")
	p := parser.New(l)
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	return prog
}

func TestFindWorkflowBlock(t *testing.T) {
	t.Run("nil_program", func(t *testing.T) {
		_, err := findWorkflowBlock(nil)
		if err == nil || !strings.Contains(err.Error(), "nil") {
			t.Fatalf("got err=%v", err)
		}
	})

	t.Run("no_workflow_block", func(t *testing.T) {
		src := `model gpt4 {
  provider = "openai"
  model_name = "gpt-4o"
}`
		prog := parseProgramStartTest(t, src)
		_, err := findWorkflowBlock(prog)
		if err == nil || !strings.Contains(err.Error(), "no workflow block") {
			t.Fatalf("got err=%v", err)
		}
	})

	t.Run("finds_workflow", func(t *testing.T) {
		src := `model gpt4 {
  provider = "openai"
  model_name = "gpt-4o"
}
agent A {
  model = gpt4
  persona = "p"
}
workflow main {
  A
}`
		prog := parseProgramStartTest(t, src)
		wf, err := findWorkflowBlock(prog)
		if err != nil {
			t.Fatal(err)
		}
		if wf.Kind != types.BlockKindWorkflow {
			t.Fatalf("Kind = %q", wf.Kind)
		}
		if wf.Name != "main" {
			t.Fatalf("Name = %q", wf.Name)
		}
	})

	t.Run("returns_first_workflow", func(t *testing.T) {
		src := `model gpt4 {
  provider = "openai"
  model_name = "gpt-4o"
}
workflow first {
}
workflow second {
}`
		prog := parseProgramStartTest(t, src)
		wf, err := findWorkflowBlock(prog)
		if err != nil {
			t.Fatal(err)
		}
		if wf.Name != "first" {
			t.Fatalf("want first workflow, got %q", wf.Name)
		}
	})

	t.Run("skips_non_block_statements", func(t *testing.T) {
		// Parser only produces BlockStatements today; future-proof by ensuring we skip unknown stmt types.
		prog := parseProgramStartTest(t, `workflow only {
}`)
		wf, err := findWorkflowBlock(prog)
		if err != nil {
			t.Fatal(err)
		}
		if wf.Name != "only" {
			t.Fatal(wf.Name)
		}
	})
}

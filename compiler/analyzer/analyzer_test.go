package analyzer

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
)

func TestAnalyzeEmptyProgram(t *testing.T) {
	program := &ast.Program{}
	diags := Analyze(program)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics, got %v", diags)
	}
}

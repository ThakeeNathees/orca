package analyzer

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
)

func TestAnalyzeEmptyProgram(t *testing.T) {
	program := &ast.Program{}
	errors := Analyze(program)
	if len(errors) != 0 {
		t.Errorf("expected no errors, got %v", errors)
	}
}

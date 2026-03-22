package codegen

import (
	"testing"

	"github.com/thakee/orca/ast"
)

func TestGenerateEmptyProgram(t *testing.T) {
	program := &ast.Program{}
	result := Generate(program)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

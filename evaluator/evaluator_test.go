package evaluator

import (
	"testing"

	"github.com/thakee/orca/ast"
)

func TestEvalReturnsNil(t *testing.T) {
	program := &ast.Program{}
	result := Eval(program)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

package workflow

import (
	"testing"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/token"
)

// ident creates an Identifier expression for testing.
func ident(name string) *ast.Identifier {
	return &ast.Identifier{Value: name}
}

// arrow creates a BinaryExpression with the ARROW operator for testing.
func arrow(left, right ast.Expression) *ast.BinaryExpression {
	return &ast.BinaryExpression{
		Left:     left,
		Operator: token.Token{Type: token.ARROW},
		Right:    right,
	}
}

func TestEdgesFromExpr(t *testing.T) {
	tests := []struct {
		name     string
		expr     ast.Expression
		expected []Edge
	}{
		{
			"simple A -> B",
			arrow(ident("A"), ident("B")),
			[]Edge{{From: "A", To: "B"}},
		},
		{
			"chain A -> B -> C",
			arrow(arrow(ident("A"), ident("B")), ident("C")),
			[]Edge{{From: "A", To: "B"}, {From: "B", To: "C"}},
		},
		{
			"non-arrow expression returns nil",
			ident("A"),
			nil,
		},
		{
			"long chain A -> B -> C -> D",
			arrow(arrow(arrow(ident("A"), ident("B")), ident("C")), ident("D")),
			[]Edge{
				{From: "A", To: "B"},
				{From: "B", To: "C"},
				{From: "C", To: "D"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EdgesFromExpr(tt.expr)
			if len(got) != len(tt.expected) {
				t.Fatalf("EdgesFromExpr() = %v, want %v", got, tt.expected)
			}
			for i, e := range got {
				if e != tt.expected[i] {
					t.Errorf("edge[%d] = %v, want %v", i, e, tt.expected[i])
				}
			}
		})
	}
}

func TestExprToNodeName(t *testing.T) {
	tests := []struct {
		name     string
		expr     ast.Expression
		expected string
	}{
		{
			"identifier",
			ident("foo"),
			"foo",
		},
		{
			"member access",
			&ast.MemberAccess{Object: ident("agents"), Member: "researcher"},
			"agents.researcher",
		},
		{
			"subscription",
			&ast.Subscription{Object: ident("agents")},
			"agents[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExprToNodeName(tt.expr)
			if got != tt.expected {
				t.Errorf("ExprToNodeName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

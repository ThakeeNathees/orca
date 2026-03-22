package ast

import (
	"testing"

	"github.com/thakee/orca/compiler/token"
)

func TestBlockStatementBaseNode(t *testing.T) {
	tests := []struct {
		name      string
		block     BlockStatement
		expStart  token.TokenType
		expLiteral string
	}{
		{
			name: "model block",
			block: BlockStatement{
				BaseNode: BaseNode{TokenStart: token.Token{Type: token.MODEL, Literal: "model"}},
				Name: "gpt4",
			},
			expStart:  token.MODEL,
			expLiteral: "model",
		},
		{
			name: "agent block",
			block: BlockStatement{
				BaseNode: BaseNode{TokenStart: token.Token{Type: token.AGENT, Literal: "agent"}},
				Name: "researcher",
			},
			expStart:  token.AGENT,
			expLiteral: "agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.block.Start().Type != tt.expStart {
				t.Errorf("expected start type %s, got %s", tt.expStart, tt.block.Start().Type)
			}
			if tt.block.Start().Literal != tt.expLiteral {
				t.Errorf("expected start literal %q, got %q", tt.expLiteral, tt.block.Start().Literal)
			}
		})
	}
}

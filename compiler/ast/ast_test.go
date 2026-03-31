package ast

import (
	"testing"

	"github.com/thakee/orca/compiler/token"
)

func TestBlockStatementBaseNode(t *testing.T) {
	tests := []struct {
		name       string
		block      BlockStatement
		expStart   token.TokenType
		expLiteral string
	}{
		{
			name: "model block",
			block: BlockStatement{
				BaseNode: BaseNode{TokenStart: token.Token{Type: token.MODEL, Literal: "model"}},
				Name:     "gpt4",
			},
			expStart:   token.MODEL,
			expLiteral: "model",
		},
		{
			name: "agent block",
			block: BlockStatement{
				BaseNode: BaseNode{TokenStart: token.Token{Type: token.AGENT, Literal: "agent"}},
				Name:     "researcher",
			},
			expStart:   token.AGENT,
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

func TestBlockStatementFieldExpression(t *testing.T) {
	prov := &StringLiteral{BaseNode: NewTerminal(token.Token{Type: token.STRING, Literal: `"openai"`}), Value: "openai"}
	idGPT := &Identifier{BaseNode: NewTerminal(token.Token{Type: token.IDENT, Literal: "gpt4"}), Value: "gpt4"}

	tests := []struct {
		name   string
		block  *BlockStatement
		field  string
		want   Expression
		wantOK bool
	}{
		{
			name: "returns value for existing field",
			block: &BlockStatement{
				Assignments: []*Assignment{
					{Name: "provider", Value: prov},
					{Name: "model", Value: idGPT},
				},
			},
			field:  "provider",
			want:   prov,
			wantOK: true,
		},
		{
			name: "second field",
			block: &BlockStatement{
				Assignments: []*Assignment{
					{Name: "provider", Value: prov},
					{Name: "model", Value: idGPT},
				},
			},
			field:  "model",
			want:   idGPT,
			wantOK: true,
		},
		{
			name: "missing field",
			block: &BlockStatement{
				Assignments: []*Assignment{{Name: "provider", Value: prov}},
			},
			field:  "temperature",
			want:   nil,
			wantOK: false,
		},
		{
			name:   "nil receiver",
			block:  nil,
			field:  "anything",
			want:   nil,
			wantOK: false,
		},
		{
			name:   "empty assignments",
			block:  &BlockStatement{Assignments: nil},
			field:  "x",
			want:   nil,
			wantOK: false,
		},
		{
			name: "first match when duplicate keys",
			block: &BlockStatement{
				Assignments: []*Assignment{
					{Name: "x", Value: prov},
					{Name: "x", Value: idGPT},
				},
			},
			field:  "x",
			want:   prov,
			wantOK: true,
		},
		{
			name: "skips nil assignment entry",
			block: &BlockStatement{
				Assignments: []*Assignment{
					nil,
					{Name: "ok", Value: prov},
				},
			},
			field:  "ok",
			want:   prov,
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.block.GetFieldExpression(tt.field)
			if ok != tt.wantOK {
				t.Fatalf("FieldExpression(%q) ok = %v, want %v", tt.field, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("FieldExpression(%q) = %p, want %p", tt.field, got, tt.want)
			}
		})
	}
}

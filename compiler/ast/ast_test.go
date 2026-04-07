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
				BaseNode: BaseNode{TokenStart: token.Token{Type: token.IDENT, Literal: "model"}},
				Name:     "gpt4",
			},
			expStart:   token.IDENT,
			expLiteral: "model",
		},
		{
			name: "agent block",
			block: BlockStatement{
				BaseNode: BaseNode{TokenStart: token.Token{Type: token.IDENT, Literal: "agent"}},
				Name:     "researcher",
			},
			expStart:   token.IDENT,
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
				BlockBody: BlockBody{Assignments: []*Assignment{
					{Name: "provider", Value: prov},
					{Name: "model", Value: idGPT},
				}},
			},
			field:  "provider",
			want:   prov,
			wantOK: true,
		},
		{
			name: "second field",
			block: &BlockStatement{
				BlockBody: BlockBody{Assignments: []*Assignment{
					{Name: "provider", Value: prov},
					{Name: "model", Value: idGPT},
				}},
			},
			field:  "model",
			want:   idGPT,
			wantOK: true,
		},
		{
			name: "missing field",
			block: &BlockStatement{
				BlockBody: BlockBody{Assignments: []*Assignment{{Name: "provider", Value: prov}}},
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
			block:  &BlockStatement{BlockBody: BlockBody{Assignments: nil}},
			field:  "x",
			want:   nil,
			wantOK: false,
		},
		{
			name: "first match when duplicate keys",
			block: &BlockStatement{
				BlockBody: BlockBody{Assignments: []*Assignment{
					{Name: "x", Value: prov},
					{Name: "x", Value: idGPT},
				}},
			},
			field:  "x",
			want:   prov,
			wantOK: true,
		},
		{
			name: "skips nil assignment entry",
			block: &BlockStatement{
				BlockBody: BlockBody{Assignments: []*Assignment{
					nil,
					{Name: "ok", Value: prov},
				}},
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

func TestWalk(t *testing.T) {
	prov := &StringLiteral{BaseNode: NewTerminal(token.Token{Type: token.STRING, Literal: `"openai"`}), Value: "openai"}
	idGPT := &Identifier{BaseNode: NewTerminal(token.Token{Type: token.IDENT, Literal: "gpt4"}), Value: "gpt4"}

	tests := []struct {
		name      string
		root      Node
		wantCount int
	}{
		{
			name:      "nil root is no-op",
			root:      nil,
			wantCount: 0,
		},
		{
			name:      "empty program",
			root:      &Program{},
			wantCount: 1,
		},
		{
			name: "block with two assignments",
			root: &Program{Statements: []Statement{
				&BlockStatement{
					BlockBody: BlockBody{
						Assignments: []*Assignment{
							{Name: "provider", Value: prov},
							{Name: "model", Value: idGPT},
						},
					},
				},
			}},
			wantCount: 6,
		},
		{
			name: "binary expression",
			root: &BinaryExpression{
				Left:  idGPT,
				Right: prov,
			},
			wantCount: 3,
		},
		{
			name: "nested inline block expression",
			root: &Program{Statements: []Statement{
				&BlockStatement{
					BlockBody: BlockBody{
						Assignments: []*Assignment{
							{
								Name: "m",
								Value: &BlockExpression{
									BlockBody: BlockBody{
										Assignments: []*Assignment{
											{Name: "provider", Value: prov},
										},
									},
								},
							},
						},
					},
				},
			}},
			wantCount: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var count int
			Walk(tt.root, func(Node) bool {
				count++
				return true
			})
			if count != tt.wantCount {
				t.Errorf("Walk count = %d, want %d", count, tt.wantCount)
			}
		})
	}
}

func TestWalk_pruneSkipsSubtree(t *testing.T) {
	inner := &StringLiteral{BaseNode: NewTerminal(token.Token{Type: token.STRING, Literal: `"x"`}), Value: "x"}
	block := &BlockStatement{
		BlockBody: BlockBody{
			Assignments: []*Assignment{
				{Name: "k", Value: inner},
			},
		},
	}
	prog := &Program{Statements: []Statement{block}}

	var sawInner bool
	Walk(prog, func(n Node) bool {
		if _, ok := n.(*BlockStatement); ok {
			return false
		}
		if n == inner {
			sawInner = true
		}
		return true
	})
	if sawInner {
		t.Error("expected inner literal not visited when BlockStatement pruned")
	}
}

func TestWalk_nilVisitor(t *testing.T) {
	prog := &Program{Statements: []Statement{
		&BlockStatement{BlockBody: BlockBody{}},
	}}
	Walk(prog, nil) // must not panic
}

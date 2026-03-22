package token

import "testing"

func TestTokenCreation(t *testing.T) {
	tok := Token{
		Type:    IDENT,
		Literal: "hello",
		Line:    1,
		Column:  5,
	}

	if tok.Type != IDENT {
		t.Errorf("expected type IDENT, got %s", tok.Type)
	}
	if tok.Literal != "hello" {
		t.Errorf("expected literal hello, got %s", tok.Literal)
	}
	if tok.Line != 1 || tok.Column != 5 {
		t.Errorf("expected position 1:5, got %d:%d", tok.Line, tok.Column)
	}
}

func TestLookupIdent(t *testing.T) {
	if LookupIdent("model") != MODEL {
		t.Errorf("expected MODEL for 'model'")
	}
	if LookupIdent("agent") != AGENT {
		t.Errorf("expected AGENT for 'agent'")
	}
	if LookupIdent("foobar") != IDENT {
		t.Errorf("expected IDENT for 'foobar'")
	}
}

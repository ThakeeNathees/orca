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

// TestDescribe exercises Describe for all token types to verify human-readable descriptions.
// Coverage: exercises Describe for all token types
func TestDescribe(t *testing.T) {
	tests := []struct {
		name     string
		tokType  TokenType
		expected string
	}{
		{"ILLEGAL", ILLEGAL, "illegal character"},
		{"EOF", EOF, "end of file"},
		{"IDENT", IDENT, "identifier"},
		{"NUMBER", NUMBER, "number"},
		{"STRING", STRING, "string"},
		{"RAWSTRING", RAWSTRING, "raw string"},
		{"ASSIGN", ASSIGN, "'='"},
		{"DOT", DOT, "'.'"},
		{"COMMA", COMMA, "','"},
		{"LBRACE", LBRACE, "'{'"},
		{"RBRACE", RBRACE, "'}'"},
		{"LBRACKET", LBRACKET, "'['"},
		{"RBRACKET", RBRACKET, "']'"},
		{"LPAREN", LPAREN, "'('"},
		{"RPAREN", RPAREN, "')'"},
		{"COLON", COLON, "':'"},
		{"PLUS", PLUS, "'+'"},
		{"MINUS", MINUS, "'-'"},
		{"STAR", STAR, "'*'"},
		{"SLASH", SLASH, "'/'"},
		{"ARROW", ARROW, "'->'"},
		{"PIPE", PIPE, "'|'"},
		{"AT", AT, "'@'"},
		{"QUESTION", QUESTION, "'?'"},
		{"unknown token", TokenType("UNKNOWN"), "'UNKNOWN'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Describe(tt.tokType); got != tt.expected {
				t.Errorf("Describe(%s) = %q, want %q", tt.tokType, got, tt.expected)
			}
		})
	}
}

// TestPrecedence verifies that operator tokens return the correct binding power
// and non-operator tokens return PrecLowest.
// Coverage: exercises Precedence for all token types
func TestPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		tokType  TokenType
		expected int
	}{
		{"ARROW", ARROW, PrecArrow},
		{"PIPE", PIPE, PrecPipe},
		{"PLUS", PLUS, PrecSum},
		{"MINUS", MINUS, PrecSum},
		{"STAR", STAR, PrecProduct},
		{"SLASH", SLASH, PrecProduct},
		{"DOT", DOT, PrecAccess},
		{"LBRACKET", LBRACKET, PrecAccess},
		{"LPAREN", LPAREN, PrecAccess},
		{"QUESTION", QUESTION, PrecTernary},
		{"IDENT lowest", IDENT, PrecLowest},
		{"EOF lowest", EOF, PrecLowest},
		{"RBRACE lowest", RBRACE, PrecLowest},
		{"ASSIGN lowest", ASSIGN, PrecLowest},
		{"COMMA lowest", COMMA, PrecLowest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Precedence(tt.tokType); got != tt.expected {
				t.Errorf("Precedence(%s) = %d, want %d", tt.tokType, got, tt.expected)
			}
		})
	}
}

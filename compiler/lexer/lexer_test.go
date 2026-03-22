package lexer

import (
	"testing"

	"github.com/thakee/orca/compiler/token"
)

func TestNew(t *testing.T) {
	l := New("abc")

	if l.input != "abc" {
		t.Errorf("expected input 'abc', got %q", l.input)
	}
	if l.ch != 'a' {
		t.Errorf("expected first char 'a', got %q", l.ch)
	}
	if l.position != 0 {
		t.Errorf("expected position 0, got %d", l.position)
	}
	if l.line != 1 {
		t.Errorf("expected line 1, got %d", l.line)
	}
}

func TestNewEmpty(t *testing.T) {
	l := New("")

	if l.ch != 0 {
		t.Errorf("expected null byte, got %q", l.ch)
	}
}

func TestReadChar(t *testing.T) {
	l := New("ab")

	if l.ch != 'a' {
		t.Fatalf("expected 'a', got %q", l.ch)
	}

	l.readChar()
	if l.ch != 'b' {
		t.Fatalf("expected 'b', got %q", l.ch)
	}

	l.readChar()
	if l.ch != 0 {
		t.Fatalf("expected 0 at EOF, got %q", l.ch)
	}
}

func TestNextTokenEOF(t *testing.T) {
	l := New("")
	tok := l.NextToken()
	if tok.Type != token.EOF {
		t.Fatalf("expected EOF, got %s", tok.Type)
	}
}

func TestNextTokenSingleChars(t *testing.T) {
	input := "={}[],.(): "

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.ASSIGN, "="},
		{token.LBRACE, "{"},
		{token.RBRACE, "}"},
		{token.LBRACKET, "["},
		{token.RBRACKET, "]"},
		{token.COMMA, ","},
		{token.DOT, "."},
		{token.LPAREN, "("},
		{token.RPAREN, ")"},
		{token.COLON, ":"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - wrong type. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - wrong literal. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextTokenSkipsWhitespace(t *testing.T) {
	input := "  =  "
	l := New(input)
	tok := l.NextToken()
	if tok.Type != token.ASSIGN {
		t.Fatalf("expected ASSIGN, got %s", tok.Type)
	}
	if tok.Column != 3 {
		t.Fatalf("expected column 3, got %d", tok.Column)
	}
}

func TestNextTokenIdentAndKeyword(t *testing.T) {
	input := "model smart_model agent"

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.MODEL, "model"},
		{token.IDENT, "smart_model"},
		{token.AGENT, "agent"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - wrong type. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - wrong literal. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextTokenString(t *testing.T) {
	input := `"hello world"`
	l := New(input)
	tok := l.NextToken()
	if tok.Type != token.STRING {
		t.Fatalf("expected STRING, got %s", tok.Type)
	}
	if tok.Literal != "hello world" {
		t.Fatalf("expected 'hello world', got %q", tok.Literal)
	}
}

func TestNextTokenNumbers(t *testing.T) {
	input := "42 0.7 .5"

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.INT, "42"},
		{token.FLOAT, "0.7"},
		{token.FLOAT, ".5"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - wrong type. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - wrong literal. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextTokenComment(t *testing.T) {
	input := `// this is a comment
model`
	l := New(input)
	tok := l.NextToken()
	// Comments are skipped, so we should get MODEL
	if tok.Type != token.MODEL {
		t.Fatalf("expected MODEL after comment, got %s (%q)", tok.Type, tok.Literal)
	}
	if tok.Line != 2 {
		t.Fatalf("expected line 2, got %d", tok.Line)
	}
}

func TestNextTokenBooleans(t *testing.T) {
	input := "true false"

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.TRUE, "true"},
		{token.FALSE, "false"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - wrong type. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - wrong literal. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextTokenOperators(t *testing.T) {
	input := "+-*/-> a->b | str|int"

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.PLUS, "+"},
		{token.MINUS, "-"},
		{token.STAR, "*"},
		{token.SLASH, "/"},
		{token.ARROW, "->"},
		{token.IDENT, "a"},
		{token.ARROW, "->"},
		{token.IDENT, "b"},
		{token.PIPE, "|"},
		{token.TYPE_STR, "str"},
		{token.PIPE, "|"},
		{token.TYPE_INT, "int"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - wrong type. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - wrong literal. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextTokenSlashVsComment(t *testing.T) {
	// A single / is division, but // starts a comment.
	input := "a / b // this is ignored"

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.IDENT, "a"},
		{token.SLASH, "/"},
		{token.IDENT, "b"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - wrong type. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - wrong literal. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNextTokenNewlineTracking(t *testing.T) {
	input := "a\nb"
	l := New(input)
	l.NextToken() // 'a'
	tok := l.NextToken()
	if tok.Line != 2 {
		t.Fatalf("expected line 2, got %d", tok.Line)
	}
}

func TestNextTokenFullBlock(t *testing.T) {
	input := `model smart_model {
  provider = "openai"
  temperature = 0.2
}`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.MODEL, "model"},
		{token.IDENT, "smart_model"},
		{token.LBRACE, "{"},
		{token.IDENT, "provider"},
		{token.ASSIGN, "="},
		{token.STRING, "openai"},
		{token.IDENT, "temperature"},
		{token.ASSIGN, "="},
		{token.FLOAT, "0.2"},
		{token.RBRACE, "}"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - wrong type. expected=%q, got=%q (literal=%q)", i, tt.expectedType, tok.Type, tok.Literal)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - wrong literal. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

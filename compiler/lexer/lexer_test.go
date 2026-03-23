package lexer

import (
	"strings"
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
	input := "model smart_model agent input null"

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.MODEL, "model"},
		{token.IDENT, "smart_model"},
		{token.AGENT, "agent"},
		{token.INPUT, "input"},
		{token.NULL, "null"},
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
		{token.IDENT, "str"},
		{token.PIPE, "|"},
		{token.IDENT, "int"},
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

// TestStringEscapeSequences verifies that escape sequences in single-line
// strings are correctly interpreted.
func TestStringEscapeSequences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"newline escape", `"hello\nworld"`, "hello\nworld"},
		{"tab escape", `"hello\tworld"`, "hello\tworld"},
		{"escaped backslash", `"path\\to"`, "path\\to"},
		{"escaped quote", `"say \"hi\""`, `say "hi"`},
		{"multiple escapes", `"a\nb\tc"`, "a\nb\tc"},
		{"line continuation", "\"foo\\\nbar\"", "foobar"},
		{"line continuation strips indent", "\"foo\\\n    bar\"", "foobar"},
		{"no escapes", `"plain"`, "plain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()
			if tok.Type != token.STRING {
				t.Fatalf("expected STRING, got %s", tok.Type)
			}
			if tok.Literal != tt.expected {
				t.Errorf("literal = %q, want %q", tok.Literal, tt.expected)
			}
		})
	}
}

// TestMultiLineString verifies that strings containing newlines are
// correctly dedented based on the closing quote's column.
func TestMultiLineString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"basic dedent",
			"\"\n    Hello\n    World\n    \"",
			"Hello\nWorld",
		},
		{
			"preserves relative indentation",
			"\"\n    Hello\n      Indented\n    World\n    \"",
			"Hello\n  Indented\nWorld",
		},
		{
			"empty lines preserved",
			"\"\n    Hello\n\n    World\n    \"",
			"Hello\n\nWorld",
		},
		{
			"no indent (closing quote at column 1)",
			"\"\nHello\nWorld\n\"",
			"Hello\nWorld",
		},
		{
			"two space baseline",
			"\"\n  line one\n    line two\n  \"",
			"line one\n  line two",
		},
		{
			"escape sequences in multi-line",
			"\"\n  hello\\tworld\n  foo\\nbar\n  \"",
			"hello\tworld\nfoo\nbar",
		},
		{
			"content on first line",
			"\"Hello\n    World\n    \"",
			"Hello\nWorld",
		},
		{
			"content on first line no indent",
			"\"Hello\nWorld\n\"",
			"Hello\nWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()
			if tok.Type != token.STRING {
				t.Fatalf("expected STRING, got %s", tok.Type)
			}
			if tok.Literal != tt.expected {
				t.Errorf("literal = %q, want %q", tok.Literal, tt.expected)
			}
		})
	}
}

// TestMultiLineStringLineTracking verifies that the lexer correctly
// tracks line numbers through multi-line strings.
func TestMultiLineStringLineTracking(t *testing.T) {
	input := "\"\n  hello\n  world\n  \"\nident"
	l := New(input)

	strTok := l.NextToken()
	if strTok.Type != token.STRING {
		t.Fatalf("expected STRING, got %s", strTok.Type)
	}
	if strTok.Line != 1 {
		t.Errorf("string line = %d, want 1", strTok.Line)
	}

	identTok := l.NextToken()
	if identTok.Type != token.IDENT {
		t.Fatalf("expected IDENT, got %s", identTok.Type)
	}
	if identTok.Line != 5 {
		t.Errorf("ident line = %d, want 5", identTok.Line)
	}
}

// TestMultiLineStringInBlock verifies multi-line strings work inside
// a block parsed by the lexer token stream.
func TestMultiLineStringInBlock(t *testing.T) {
	input := strings.Join([]string{
		`agent a {`,
		`  prompt = "`,
		`    You are a helpful assistant.`,
		`    Be concise.`,
		`    "`,
		`}`,
	}, "\n")

	l := New(input)
	// agent
	tok := l.NextToken()
	if tok.Type != token.AGENT {
		t.Fatalf("expected AGENT, got %s", tok.Type)
	}
	// a
	tok = l.NextToken()
	if tok.Type != token.IDENT {
		t.Fatalf("expected IDENT, got %s", tok.Type)
	}
	// {
	tok = l.NextToken()
	if tok.Type != token.LBRACE {
		t.Fatalf("expected LBRACE, got %s", tok.Type)
	}
	// prompt
	tok = l.NextToken()
	if tok.Type != token.IDENT {
		t.Fatalf("expected IDENT 'prompt', got %s %q", tok.Type, tok.Literal)
	}
	// =
	tok = l.NextToken()
	if tok.Type != token.ASSIGN {
		t.Fatalf("expected ASSIGN, got %s", tok.Type)
	}
	// the multi-line string
	tok = l.NextToken()
	if tok.Type != token.STRING {
		t.Fatalf("expected STRING, got %s", tok.Type)
	}
	expected := "You are a helpful assistant.\nBe concise."
	if tok.Literal != expected {
		t.Errorf("literal = %q, want %q", tok.Literal, expected)
	}
	// }
	tok = l.NextToken()
	if tok.Type != token.RBRACE {
		t.Fatalf("expected RBRACE, got %s", tok.Type)
	}
}

// TestNextTokenAnnotation verifies that @ is lexed as an AT token
// and that annotation-like sequences produce the expected token stream.
func TestNextTokenAnnotation(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []struct {
			typ token.TokenType
			lit string
		}
	}{
		{
			"bare annotation",
			"@sensitive",
			[]struct {
				typ token.TokenType
				lit string
			}{
				{token.AT, "@"},
				{token.IDENT, "sensitive"},
			},
		},
		{
			"annotation with string arg",
			`@desc("hello")`,
			[]struct {
				typ token.TokenType
				lit string
			}{
				{token.AT, "@"},
				{token.IDENT, "desc"},
				{token.LPAREN, "("},
				{token.STRING, "hello"},
				{token.RPAREN, ")"},
			},
		},
		{
			"at sign position tracking",
			"@x",
			[]struct {
				typ token.TokenType
				lit string
			}{
				{token.AT, "@"},
				{token.IDENT, "x"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input)
			for i, exp := range tt.expect {
				tok := l.NextToken()
				if tok.Type != exp.typ {
					t.Fatalf("token[%d] type = %q, want %q", i, tok.Type, exp.typ)
				}
				if tok.Literal != exp.lit {
					t.Fatalf("token[%d] literal = %q, want %q", i, tok.Literal, exp.lit)
				}
			}
		})
	}
}

// TestNextTokenAtPosition verifies that the AT token has correct position.
func TestNextTokenAtPosition(t *testing.T) {
	l := New("  @desc")
	tok := l.NextToken()
	if tok.Type != token.AT {
		t.Fatalf("expected AT, got %s", tok.Type)
	}
	if tok.Column != 3 {
		t.Errorf("AT column = %d, want 3", tok.Column)
	}
}

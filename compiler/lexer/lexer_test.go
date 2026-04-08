package lexer

import (
	"strings"
	"testing"

	"github.com/thakee/orca/compiler/token"
)

func TestNew(t *testing.T) {
	l := New("abc", "")

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
	l := New("", "")

	if l.ch != 0 {
		t.Errorf("expected null byte, got %q", l.ch)
	}
}

func TestReadChar(t *testing.T) {
	l := New("ab", "")

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
	l := New("", "")
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

	l := New(input, "")
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
	l := New(input, "")
	tok := l.NextToken()
	if tok.Type != token.ASSIGN {
		t.Fatalf("expected ASSIGN, got %s", tok.Type)
	}
	if tok.Column != 3 {
		t.Fatalf("expected column 3, got %d", tok.Column)
	}
}

// Block kind names (model, agent, …) are ordinary identifiers at lex time.
func TestNextTokenIdentAndKeyword(t *testing.T) {
	input := "model smart_model agent input null"

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.IDENT, "model"},
		{token.IDENT, "smart_model"},
		{token.IDENT, "agent"},
		{token.IDENT, "input"},
		{token.IDENT, "null"},
		{token.EOF, ""},
	}

	l := New(input, "")
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

// TestNextTokenCronWebhook verifies former block names are lexed as identifiers.
func TestNextTokenCronWebhook(t *testing.T) {
	input := "cron daily webhook hooks_in"

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.IDENT, "cron"},
		{token.IDENT, "daily"},
		{token.IDENT, "webhook"},
		{token.IDENT, "hooks_in"},
		{token.EOF, ""},
	}

	l := New(input, "")
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
	l := New(input, "")
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
		{token.NUMBER, "42"},
		{token.NUMBER, "0.7"},
		{token.NUMBER, ".5"},
		{token.EOF, ""},
	}

	l := New(input, "")
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
	l := New(input, "")
	tok := l.NextToken()
	// Comments are skipped, so we should get the identifier "model"
	if tok.Type != token.IDENT || tok.Literal != "model" {
		t.Fatalf("expected IDENT \"model\" after comment, got %s (%q)", tok.Type, tok.Literal)
	}
	if tok.Line != 2 {
		t.Fatalf("expected line 2, got %d", tok.Line)
	}
}

// Lexer does not classify boolean spellings — they are IDENT like other names.
func TestNextTokenBooleans(t *testing.T) {
	input := "true false"

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.IDENT, "true"},
		{token.IDENT, "false"},
		{token.EOF, ""},
	}

	l := New(input, "")
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
	input := "+-*/-> a->b | string|int"

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
		{token.IDENT, "string"},
		{token.PIPE, "|"},
		{token.IDENT, "int"},
		{token.EOF, ""},
	}

	l := New(input, "")
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

	l := New(input, "")
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
	l := New(input, "")
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
		{token.IDENT, "model"},
		{token.IDENT, "smart_model"},
		{token.LBRACE, "{"},
		{token.IDENT, "provider"},
		{token.ASSIGN, "="},
		{token.STRING, "openai"},
		{token.IDENT, "temperature"},
		{token.ASSIGN, "="},
		{token.NUMBER, "0.2"},
		{token.RBRACE, "}"},
		{token.EOF, ""},
	}

	l := New(input, "")
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
		{"no escapes", `"plain"`, "plain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input, "")
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

// TestRawString verifies that triple-backtick raw strings are correctly
// lexed and dedented based on the closing backtick's column.
// TestRawString verifies that triple-backtick raw strings are correctly
// lexed and dedented. The Literal format is "lang\ncontent".
func TestRawString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // full Literal including "lang\n" prefix
	}{
		{
			"basic dedent",
			"```\n    Hello\n    World\n    ```",
			"\nHello\nWorld",
		},
		{
			"with language tag",
			"```md\n    Hello\n    World\n    ```",
			"md\nHello\nWorld",
		},
		{
			"preserves relative indentation",
			"```\n    Hello\n      Indented\n    World\n    ```",
			"\nHello\n  Indented\nWorld",
		},
		{
			"empty lines preserved",
			"```\n    Hello\n\n    World\n    ```",
			"\nHello\n\nWorld",
		},
		{
			"no indent (closing at column 1)",
			"```\nHello\nWorld\n```",
			"\nHello\nWorld",
		},
		{
			"two space baseline",
			"```\n  line one\n    line two\n  ```",
			"\nline one\n  line two",
		},
		{
			"python language tag",
			"```py\nprint('hello')\n```",
			"py\nprint('hello')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.input, "")
			tok := l.NextToken()
			if tok.Type != token.RAWSTRING {
				t.Fatalf("expected RAWSTRING, got %s", tok.Type)
			}
			if tok.Literal != tt.expected {
				t.Errorf("literal = %q, want %q", tok.Literal, tt.expected)
			}
		})
	}
}

// TestRawStringLineTracking verifies that the lexer correctly
// tracks line numbers through raw strings.
func TestRawStringLineTracking(t *testing.T) {
	input := "```\n  hello\n  world\n```\nident"
	l := New(input, "")

	strTok := l.NextToken()
	if strTok.Type != token.RAWSTRING {
		t.Fatalf("expected RAWSTRING, got %s", strTok.Type)
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

// TestRawStringInBlock verifies raw strings work inside a block.
// Closing ``` at 4 spaces indent strips 4 spaces from content.
func TestRawStringInBlock(t *testing.T) {
	input := strings.Join([]string{
		"agent a {",
		"    prompt = ```md",
		"        You are a helpful assistant.",
		"        Be concise.",
		"        ```",
		"}",
	}, "\n")

	l := New(input, "")
	// agent (block kind name — lexed as IDENT)
	tok := l.NextToken()
	if tok.Type != token.IDENT || tok.Literal != "agent" {
		t.Fatalf("expected IDENT \"agent\", got %s %q", tok.Type, tok.Literal)
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
	// the raw string (Literal format: "lang\ncontent")
	tok = l.NextToken()
	if tok.Type != token.RAWSTRING {
		t.Fatalf("expected RAWSTRING, got %s", tok.Type)
	}
	expected := "md\nYou are a helpful assistant.\nBe concise."
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
			l := New(tt.input, "")
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

// TestStringIsSingleLineOnly verifies that double-quoted strings stop at newlines.
func TestStringIsSingleLineOnly(t *testing.T) {
	// A newline inside a double-quoted string terminates the string.
	input := "\"hello\nworld\""
	l := New(input, "")
	tok := l.NextToken()
	if tok.Type != token.STRING {
		t.Fatalf("expected STRING, got %s", tok.Type)
	}
	if tok.Literal != "hello" {
		t.Errorf("literal = %q, want %q", tok.Literal, "hello")
	}
}

// TestRawStringUnterminatedEOF verifies that an unterminated raw string
// reads until EOF without crashing.
func TestRawStringUnterminatedEOF(t *testing.T) {
	input := "```\nhello\nworld"
	l := New(input, "")
	tok := l.NextToken()
	if tok.Type != token.RAWSTRING {
		t.Fatalf("expected RAWSTRING, got %s", tok.Type)
	}
}

// TestRawStringPositionTracking verifies start/end positions on raw strings.
func TestRawStringPositionTracking(t *testing.T) {
	input := "  ```md\n    content\n  ```"
	l := New(input, "")
	tok := l.NextToken()
	if tok.Type != token.RAWSTRING {
		t.Fatalf("expected RAWSTRING, got %s", tok.Type)
	}
	if tok.Line != 1 {
		t.Errorf("Line = %d, want 1", tok.Line)
	}
	if tok.Column != 3 {
		t.Errorf("Column = %d, want 3", tok.Column)
	}
	if tok.EndLine != 3 {
		t.Errorf("EndLine = %d, want 3", tok.EndLine)
	}
}

// TestSingleBacktickIsIllegal verifies that a lone ` is an ILLEGAL token.
func TestSingleBacktickIsIllegal(t *testing.T) {
	l := New("`", "")
	tok := l.NextToken()
	if tok.Type != token.ILLEGAL {
		t.Fatalf("expected ILLEGAL, got %s", tok.Type)
	}
}

// TestDoubleBacktickIsIllegal verifies that an empty pair of backticks yields two ILLEGAL tokens.
func TestDoubleBacktickIsIllegal(t *testing.T) {
	l := New("``", "")
	tok := l.NextToken()
	if tok.Type != token.ILLEGAL {
		t.Fatalf("expected ILLEGAL, got %s", tok.Type)
	}
}

// TestRawStringFollowedByTokens verifies tokens after a raw string are correct.
func TestRawStringFollowedByTokens(t *testing.T) {
	input := "```\nfoo\n``` = 42"
	l := New(input, "")

	tok := l.NextToken()
	if tok.Type != token.RAWSTRING {
		t.Fatalf("expected RAWSTRING, got %s", tok.Type)
	}
	if tok.Literal != "\nfoo" {
		t.Errorf("literal = %q, want %q", tok.Literal, "\nfoo")
	}

	tok = l.NextToken()
	if tok.Type != token.ASSIGN {
		t.Fatalf("expected ASSIGN, got %s (%q)", tok.Type, tok.Literal)
	}

	tok = l.NextToken()
	if tok.Type != token.NUMBER {
		t.Fatalf("expected NUMBER, got %s (%q)", tok.Type, tok.Literal)
	}
	if tok.Literal != "42" {
		t.Errorf("literal = %q, want %q", tok.Literal, "42")
	}
}

// TestNextTokenAtPosition verifies that the AT token has correct position.
func TestNextTokenAtPosition(t *testing.T) {
	l := New("  @desc", "")
	tok := l.NextToken()
	if tok.Type != token.AT {
		t.Fatalf("expected AT, got %s", tok.Type)
	}
	if tok.Column != 3 {
		t.Errorf("AT column = %d, want 3", tok.Column)
	}
}

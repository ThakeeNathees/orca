// Package lexer implements the tokenizer for the Orca language.
// It reads raw .oc source text character by character and produces
// a stream of tokens for the parser. Handles strings, numbers (int/float),
// identifiers/keywords, single-char operators, and comments.
package lexer

import "github.com/thakee/orca/compiler/token"

// Lexer holds the state for scanning through an input string.
// It tracks the current read position and source location (line/column)
// for accurate token positioning.
type Lexer struct {
	input        string
	position     int  // current position in input (points to current char)
	readPosition int  // next position to read (one ahead of position)
	ch           byte // current character under examination
	line         int  // current line number (1-based)
	column       int  // current column number (1-based)
}

// New creates a Lexer for the given input string and primes it
// by reading the first character. Line starts at 1, column at 0
// because readChar increments column before the first real read.
func New(input string) *Lexer {
	l := &Lexer{input: input, line: 1, column: 0}
	l.readChar()
	return l
}

// readChar advances the lexer by one character. Sets ch to 0 (NUL)
// when the end of input is reached, which signals EOF to the rest
// of the lexer.
func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
	l.column++
}

// peekChar returns the next character without advancing the lexer.
// Used for lookahead decisions (e.g., distinguishing "." from ".5").
func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

// NextToken reads and returns the next token from the input.
// Skips whitespace and comments before identifying the token.
// Returns an EOF token when the input is exhausted.
func (l *Lexer) NextToken() token.Token {
	l.skipWhitespace()
	l.skipComment()

	tok := token.Token{Line: l.line, Column: l.column}

	switch l.ch {
	case '=':
		tok.Type = token.ASSIGN
		tok.Literal = "="
	case '{':
		tok.Type = token.LBRACE
		tok.Literal = "{"
	case '}':
		tok.Type = token.RBRACE
		tok.Literal = "}"
	case '[':
		tok.Type = token.LBRACKET
		tok.Literal = "["
	case ']':
		tok.Type = token.RBRACKET
		tok.Literal = "]"
	case '(':
		tok.Type = token.LPAREN
		tok.Literal = "("
	case ')':
		tok.Type = token.RPAREN
		tok.Literal = ")"
	case ':':
		tok.Type = token.COLON
		tok.Literal = ":"
	case ',':
		tok.Type = token.COMMA
		tok.Literal = ","
	case '+':
		tok.Type = token.PLUS
		tok.Literal = "+"
	case '-':
		// -> is the arrow operator, otherwise - is minus.
		if l.peekChar() == '>' {
			l.readChar()
			tok.Type = token.ARROW
			tok.Literal = "->"
		} else {
			tok.Type = token.MINUS
			tok.Literal = "-"
		}
	case '|':
		tok.Type = token.PIPE
		tok.Literal = "|"
	case '*':
		tok.Type = token.STAR
		tok.Literal = "*"
	case '/':
		// // starts a comment, otherwise / is division.
		if l.peekChar() == '/' {
			// Let skipComment handle it — but we've already passed
			// skipComment at the top of NextToken, so handle inline.
			for l.ch != '\n' && l.ch != 0 {
				l.readChar()
			}
			// Recurse to get the actual next token.
			return l.NextToken()
		}
		tok.Type = token.SLASH
		tok.Literal = "/"
	case '.':
		// A dot followed by a digit starts a float literal like .5,
		// otherwise it's a standalone dot operator.
		if isDigit(l.peekChar()) {
			return l.readNumber()
		}
		tok.Type = token.DOT
		tok.Literal = "."
	case '"':
		tok.Type = token.STRING
		tok.Literal = l.readString()
		return tok
	case 0:
		tok.Type = token.EOF
		tok.Literal = ""
		return tok
	default:
		if isLetter(l.ch) {
			// Read the full identifier, then check if it's a keyword.
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			return tok
		} else if isDigit(l.ch) {
			return l.readNumber()
		}
		tok.Type = token.ILLEGAL
		tok.Literal = string(l.ch)
	}

	l.readChar()
	return tok
}

// skipWhitespace consumes spaces, tabs, carriage returns, and newlines.
// Tracks line increments on newline characters so token positions stay accurate.
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' || l.ch == '\n' {
		if l.ch == '\n' {
			l.line++
			l.column = 0
		}
		l.readChar()
	}
}

// skipComment skips single-line comments starting with //.
// Consumes everything until end-of-line or end-of-input, then
// recurses through skipWhitespace/skipComment to handle consecutive
// comment lines and blank lines between comments.
func (l *Lexer) skipComment() {
	if l.ch == '/' && l.peekChar() == '/' {
		for l.ch != '\n' && l.ch != 0 {
			l.readChar()
		}
		l.skipWhitespace()
		l.skipComment()
	}
}

// readIdentifier reads a contiguous sequence of letters and digits
// starting at the current position. Identifiers must start with a letter
// (enforced by the caller in NextToken), but can contain digits after.
func (l *Lexer) readIdentifier() string {
	pos := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[pos:l.position]
}

// readString reads a double-quoted string, consuming everything between
// the opening and closing quote. Returns the content without quotes.
// Does not currently handle escape sequences.
func (l *Lexer) readString() string {
	l.readChar() // skip opening quote
	pos := l.position
	for l.ch != '"' && l.ch != 0 {
		l.readChar()
	}
	s := l.input[pos:l.position]
	l.readChar() // skip closing quote
	return s
}

// readNumber reads an integer or float literal. Handles numbers starting
// with a digit or a dot (e.g., ".5"). If a dot is encountered mid-number,
// the token is classified as FLOAT; otherwise INT.
func (l *Lexer) readNumber() token.Token {
	tok := token.Token{Line: l.line, Column: l.column}
	pos := l.position
	isFloat := l.ch == '.'

	for isDigit(l.ch) || (l.ch == '.' && isDigit(l.peekChar())) {
		if l.ch == '.' {
			isFloat = true
		}
		l.readChar()
	}

	tok.Literal = l.input[pos:l.position]
	if isFloat {
		tok.Type = token.FLOAT
	} else {
		tok.Type = token.INT
	}
	return tok
}

// isLetter returns true for ASCII letters and underscore.
// Underscores are allowed in identifiers (e.g., web_search).
func isLetter(ch byte) bool {
	return ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '_'
}

// isDigit returns true for ASCII digit characters 0-9.
func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

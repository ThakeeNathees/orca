// Package lexer implements the tokenizer for the Orca language.
// It reads raw .oc source text character by character and produces
// a stream of tokens for the parser. Handles strings, numbers (int/float),
// identifiers/keywords, single-char operators, and comments.
package lexer

import (
	"strings"

	"github.com/thakee/orca/compiler/token"
)

// Lexer holds the state for scanning through an input string.
// It tracks the current read position and source location (line/column)
// for accurate token positioning.
type Lexer struct {
	input        string
	position     int    // current position in input (points to current char)
	readPosition int    // next position to read (one ahead of position)
	ch           byte   // current character under examination
	line         int    // current line number (1-based)
	column       int    // current column number (1-based)
	SourceFile   string // the .oc file being lexed; empty for in-memory/test inputs
}

// New creates a Lexer for the given input string and primes it
// by reading the first character. Line starts at 1, column at 0
// because readChar increments column before the first real read.
// sourceFile is the path of the .oc file being lexed; pass "" for
// in-memory or test inputs.
func New(input string, sourceFile string) *Lexer {
	l := &Lexer{input: input, line: 1, column: 0, SourceFile: sourceFile}
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
	case '@':
		tok.Type = token.AT
		tok.Literal = "@"
	case '?':
		tok.Type = token.QUESTION
		tok.Literal = "?"
	case '\\':
		tok.Type = token.BACKSLASH
		tok.Literal = "\\"
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
	case '`':
		// Triple backtick starts a raw multi-line string.
		if l.peekIsTripleBacktick() {
			tok.Type = token.RAWSTRING
			tok.Literal = l.readRawString()
			tok.EndLine = l.line
			tok.EndCol = l.column - 1
			return tok
		}
		tok.Type = token.ILLEGAL
		tok.Literal = string(l.ch)
	case '"':
		tok.Type = token.STRING
		tok.Literal = l.readString()
		// String end position: lexer is past the closing quote.
		tok.EndLine = l.line
		tok.EndCol = l.column - 1
		return tok
	case 0:
		tok.Type = token.EOF
		tok.Literal = ""
		tok.EndLine = tok.Line
		tok.EndCol = tok.Column
		return tok
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = token.IDENT
			setTokenEnd(&tok)
			return tok
		} else if isDigit(l.ch) {
			return l.readNumber()
		}
		tok.Type = token.ILLEGAL
		tok.Literal = string(l.ch)
	}

	setTokenEnd(&tok)
	l.readChar()
	return tok
}

// setTokenEnd sets EndLine/EndCol based on the token's literal length.
// For single-line tokens, the end is at Column + len(Literal) - 1.
func setTokenEnd(tok *token.Token) {
	tok.EndLine = tok.Line
	tok.EndCol = tok.Column + len(tok.Literal) - 1
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

// readString reads a double-quoted single-line string with escape sequence
// support (\n, \t, \\, \"). Newlines are not allowed — use triple-backtick
// raw strings for multi-line content.
func (l *Lexer) readString() string {
	l.readChar() // skip opening quote

	var sb strings.Builder
	for l.ch != '"' && l.ch != '\n' && l.ch != 0 {
		if l.ch == '\\' {
			l.readChar() // skip backslash
			switch l.ch {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			default:
				sb.WriteByte('\\')
				sb.WriteByte(l.ch)
			}
		} else {
			sb.WriteByte(l.ch)
		}
		l.readChar()
	}

	l.readChar() // skip closing quote
	return sb.String()
}

// readRawString reads a triple-backtick delimited raw string (``` ... ```).
// The opening backticks may be followed by an optional language tag (e.g. ```md).
// Content is auto-dedented based on the closing ```'s column position.
// Sets l.RawStringLang to the language tag (or empty string).
func (l *Lexer) readRawString() string {
	// Skip the three opening backticks (l.ch is on the first `)
	l.readChar() // move to second `
	l.readChar() // move to third `
	l.readChar() // move past third `

	// Read optional language tag (letters/digits until newline or whitespace).
	var lang strings.Builder
	for l.ch != '\n' && l.ch != 0 && l.ch != ' ' && l.ch != '\t' && l.ch != '\r' {
		lang.WriteByte(l.ch)
		l.readChar()
	}

	// Skip the rest of the opening line (whitespace after lang tag).
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' {
		l.readChar()
	}
	// Skip the newline after opening backticks.
	if l.ch == '\n' {
		l.line++
		l.column = 0
		l.readChar()
	}

	// Read content until closing ```.
	var sb strings.Builder
	for l.ch != 0 {
		if l.ch == '`' && l.peekIsTripleBacktick() {
			break
		}
		if l.ch == '\n' {
			l.line++
			l.column = 0
		}
		sb.WriteByte(l.ch)
		l.readChar()
	}

	// The closing ``` column is the baseline for dedenting.
	baseline := l.column - 1

	// Skip the three closing backticks.
	l.readChar() // skip first `
	l.readChar() // skip second `
	l.readChar() // skip third `

	raw := sb.String()
	return lang.String() + "\n" + dedentRawString(raw, baseline)
}

// peekIsTripleBacktick checks if the current char and next two chars form ```.
func (l *Lexer) peekIsTripleBacktick() bool {
	if l.ch != '`' {
		return false
	}
	if l.readPosition >= len(l.input) {
		return false
	}
	if l.input[l.readPosition] != '`' {
		return false
	}
	if l.readPosition+1 >= len(l.input) {
		return false
	}
	return l.input[l.readPosition+1] == '`'
}

// dedentRawString strips baseline indentation from a raw string's content.
// Removes the trailing line (whitespace before closing ```), and strips
// up to baseline leading spaces from each remaining line.
func dedentRawString(raw string, baseline int) string {
	lines := strings.Split(raw, "\n")

	// Remove the last line (whitespace before closing ```).
	if len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}

	// Strip baseline indentation from each line.
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		strip := baseline
		if strip > len(line) {
			strip = len(line)
		}
		j := 0
		for j < strip && j < len(line) && line[j] == ' ' {
			j++
		}
		lines[i] = line[j:]
	}

	return strings.Join(lines, "\n")
}

// readNumber reads an integer or float literal. Handles numbers starting
// with a digit or a dot (e.g., ".5"). If a dot is encountered mid-number,
// the token is classified as FLOAT; otherwise INT.
func (l *Lexer) readNumber() token.Token {
	tok := token.Token{Line: l.line, Column: l.column}
	pos := l.position

	for isDigit(l.ch) || (l.ch == '.' && isDigit(l.peekChar())) {
		l.readChar()
	}

	tok.Type = token.NUMBER
	tok.Literal = l.input[pos:l.position]
	setTokenEnd(&tok)
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

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
	case '@':
		tok.Type = token.AT
		tok.Literal = "@"
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
			tok.Type = token.LookupIdent(tok.Literal)
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

// readString reads a double-quoted string with escape sequence support
// (\n, \t, \\, \"). All strings can span multiple lines. If the string
// contains newlines, the closing quote's column defines the baseline
// indentation to strip. For single-line strings (no newlines), the
// content is returned as-is after escape processing.
func (l *Lexer) readString() string {
	l.readChar() // skip opening quote

	isMultiLine := false
	var sb strings.Builder
	for l.ch != '"' && l.ch != 0 {
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
			case '\n':
				// Line continuation: backslash-newline joins lines.
				// Skip the newline and any leading whitespace on the next line.
				l.line++
				l.column = 0
				l.readChar()
				for l.ch == ' ' || l.ch == '\t' {
					l.readChar()
				}
				continue // skip the l.readChar() at the end of the loop
			default:
				sb.WriteByte('\\')
				sb.WriteByte(l.ch)
			}
		} else {
			if l.ch == '\n' {
				isMultiLine = true
				l.line++
				l.column = 0
			}
			sb.WriteByte(l.ch)
		}
		l.readChar()
	}

	// For multi-line strings, use the closing quote's column as baseline.
	baseline := l.column - 1
	l.readChar() // skip closing quote

	raw := sb.String()
	if isMultiLine {
		return dedentString(raw, baseline)
	}
	return raw
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
	setTokenEnd(&tok)
	return tok
}

// dedentString strips baseline indentation from a multi-line string's
// raw content. Removes the first line if empty (right after opening quote's
// newline), removes the trailing line (whitespace before closing quote),
// and strips up to baseline leading spaces from each remaining line.
// Empty lines are preserved as empty.
func dedentString(raw string, baseline int) string {
	lines := strings.Split(raw, "\n")

	// Remove the first line if it's empty (opening " followed by newline).
	if len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}

	// Remove the last line (whitespace before closing quote).
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
		// Only strip space characters (don't strip content).
		j := 0
		for j < strip && j < len(line) && line[j] == ' ' {
			j++
		}
		lines[i] = line[j:]
	}

	return strings.Join(lines, "\n")
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

// Package token defines the token types and Token struct used throughout
// the Orca lexer and parser. Each token carries its type, literal text,
// and source position (line/column) for error reporting and source mapping.
package token

// TokenType identifies what kind of token this is (keyword, literal, operator, etc.).
type TokenType string

const (
	// Special tokens
	ILLEGAL TokenType = "ILLEGAL" // unrecognized character
	EOF     TokenType = "EOF"     // end of input

	// Identifiers + Literals
	IDENT     TokenType = "IDENT"     // user-defined names (variable names, block references)
	NUMBER    TokenType = "NUMBER"    // number literal: 42, 3.14, etc.
	STRING    TokenType = "STRING"    // string literal: "hello"
	RAWSTRING TokenType = "RAWSTRING" // raw multi-line string: ```md ... ```

	// Operators & Delimiters
	ASSIGN   TokenType = "="
	DOT      TokenType = "."
	COMMA    TokenType = ","
	LBRACE   TokenType = "{"
	RBRACE   TokenType = "}"
	LBRACKET TokenType = "["
	RBRACKET TokenType = "]"
	LPAREN   TokenType = "("
	RPAREN   TokenType = ")"
	COLON    TokenType = ":"
	PLUS     TokenType = "+"
	MINUS    TokenType = "-"
	STAR     TokenType = "*"
	SLASH    TokenType = "/"
	ARROW    TokenType = "->"
	PIPE     TokenType = "|"
	AT       TokenType = "@"
	QUESTION TokenType = "?"
)

// Operator precedence levels for Pratt parsing. Higher values bind tighter.
const (
	PrecLowest  int = iota
	PrecTernary     // ?:
	PrecArrow       // ->
	PrecPipe        // |
	PrecSum         // + -
	PrecProduct     // * /
	PrecAccess      // .
)

// Token represents a single lexical token with its type, literal text,
// and source position. Line and Column enable source mapping from generated
// code back to the original .oc file.
type Token struct {
	Type    TokenType
	Literal string
	Line    int // 1-based line number in source
	Column  int // 1-based column number in source
	EndLine int // end position for multi-line tokens (0 means same as Line)
	EndCol  int // end column for multi-line tokens (0 means same as Column)
}

// Describe returns a human-readable description of a token type,
// suitable for use in error messages. E.g., IDENT → "identifier",
// LBRACE → "'{'", STRING → "string".
func Describe(t TokenType) string {
	switch t {
	case ILLEGAL:
		return "illegal character"
	case EOF:
		return "end of file"
	case IDENT:
		return "identifier"
	case NUMBER:
		return "number"
	case STRING:
		return "string"
	case RAWSTRING:
		return "raw string"
	case ASSIGN:
		return "'='"
	case DOT:
		return "'.'"
	case COMMA:
		return "','"
	case LBRACE:
		return "'{'"
	case RBRACE:
		return "'}'"
	case LBRACKET:
		return "'['"
	case RBRACKET:
		return "']'"
	case LPAREN:
		return "'('"
	case RPAREN:
		return "')'"
	case COLON:
		return "':'"
	case PLUS:
		return "'+'"
	case MINUS:
		return "'-'"
	case STAR:
		return "'*'"
	case SLASH:
		return "'/'"
	case ARROW:
		return "'->'"
	case PIPE:
		return "'|'"
	case AT:
		return "'@'"
	case QUESTION:
		return "'?'"
	default:
		return "'" + string(t) + "'"
	}
}

// Precedence returns the binding power of a token type when used as a
// binary operator. Returns PrecLowest for non-operator tokens, which
// stops the Pratt parsing loop.
func Precedence(t TokenType) int {
	switch t {
	case ARROW:
		return PrecArrow
	case PIPE:
		return PrecPipe
	case PLUS, MINUS:
		return PrecSum
	case STAR, SLASH:
		return PrecProduct
	case QUESTION:
		return PrecTernary
	case DOT, LBRACKET, LPAREN:
		return PrecAccess
	default:
		return PrecLowest
	}
}

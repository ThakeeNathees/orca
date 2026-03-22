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
	IDENT  TokenType = "IDENT"  // user-defined names (variable names, block references)
	INT    TokenType = "INT"    // integer literal: 123
	FLOAT  TokenType = "FLOAT"  // float literal: 0.2
	STRING TokenType = "STRING" // string literal: "hello"

	// Operators & Delimiters
	ASSIGN   TokenType = "="
	DOT      TokenType = "."
	COMMA    TokenType = ","
	LBRACE   TokenType = "{"
	RBRACE   TokenType = "}"
	LBRACKET TokenType = "["
	RBRACKET TokenType = "]"
	PLUS     TokenType = "+"
	MINUS    TokenType = "-"
	STAR     TokenType = "*"
	SLASH    TokenType = "/"
	ARROW    TokenType = "->"

	// Boolean literals
	TRUE  TokenType = "TRUE"
	FALSE TokenType = "FALSE"

	// Keywords — each corresponds to a top-level block type in Orca syntax.
	MODEL     TokenType = "MODEL"
	AGENT     TokenType = "AGENT"
	TASK      TokenType = "TASK"
	KNOWLEDGE TokenType = "KNOWLEDGE"
	TRIGGER   TokenType = "TRIGGER"
	WORKFLOW  TokenType = "WORKFLOW"
	TOOL      TokenType = "TOOL"
)

// Operator precedence levels for Pratt parsing. Higher values bind tighter.
const (
	PrecLowest  int = iota
	PrecArrow       // ->
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
	case INT:
		return "integer"
	case FLOAT:
		return "number"
	case STRING:
		return "string"
	case TRUE, FALSE:
		return "boolean"
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
	default:
		// Keywords like MODEL, AGENT, etc.
		if IsBlockKeyword(t) {
			return "'" + string(t) + "'"
		}
		return string(t)
	}
}

// keywords maps lowercase keyword strings to their token types.
// Used by LookupIdent to distinguish keywords from regular identifiers.
var keywords = map[string]TokenType{
	"true":      TRUE,
	"false":     FALSE,
	"model":     MODEL,
	"agent":     AGENT,
	"task":      TASK,
	"knowledge": KNOWLEDGE,
	"trigger":   TRIGGER,
	"workflow":  WORKFLOW,
	"tool":      TOOL,
}

// LookupIdent checks if an identifier string is a reserved keyword.
// Returns the keyword's TokenType if found, otherwise returns IDENT.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}

// IsBlockKeyword returns true if the token type introduces a block
// (model, agent, tool, task, knowledge, trigger, workflow).
func IsBlockKeyword(t TokenType) bool {
	switch t {
	case MODEL, AGENT, TASK, KNOWLEDGE, TRIGGER, WORKFLOW, TOOL:
		return true
	}
	return false
}

// IsIdentLike returns true if the token type can serve as an identifier
// in contexts like assignment keys. Block keywords (model, agent, etc.)
// are valid key names inside blocks — e.g., `model = gpt4` inside an
// agent block uses "model" as a key.
func IsIdentLike(t TokenType) bool {
	return t == IDENT || IsBlockKeyword(t)
}

// Precedence returns the binding power of a token type when used as a
// binary operator. Returns PrecLowest for non-operator tokens, which
// stops the Pratt parsing loop.
func Precedence(t TokenType) int {
	switch t {
	case ARROW:
		return PrecArrow
	case PLUS, MINUS:
		return PrecSum
	case STAR, SLASH:
		return PrecProduct
	case DOT:
		return PrecAccess
	default:
		return PrecLowest
	}
}

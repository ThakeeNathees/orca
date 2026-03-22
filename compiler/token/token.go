// Package token defines the token types and Token struct used throughout
// the Orca lexer and parser. Each token carries its type, literal text,
// and source position (line/column) for error reporting and source mapping.
package token

import "strings"

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
	LPAREN   TokenType = "("
	RPAREN   TokenType = ")"
	COLON    TokenType = ":"
	PLUS     TokenType = "+"
	MINUS    TokenType = "-"
	STAR     TokenType = "*"
	SLASH    TokenType = "/"
	ARROW    TokenType = "->"
	PIPE     TokenType = "|"

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
	SCHEMA    TokenType = "SCHEMA"

	// Type annotation keywords — used in type = <annotation> assignments.
	TYPE_STR   TokenType = "TYPE_STR"
	TYPE_INT   TokenType = "TYPE_INT"
	TYPE_FLOAT TokenType = "TYPE_FLOAT"
	TYPE_BOOL  TokenType = "TYPE_BOOL"
	TYPE_LIST  TokenType = "TYPE_LIST"
	TYPE_MAP   TokenType = "TYPE_MAP"
	TYPE_ANY   TokenType = "TYPE_ANY"
)

// Operator precedence levels for Pratt parsing. Higher values bind tighter.
const (
	PrecLowest  int = iota
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
	case TYPE_STR:
		return "type 'str'"
	case TYPE_INT:
		return "type 'int'"
	case TYPE_FLOAT:
		return "type 'float'"
	case TYPE_BOOL:
		return "type 'bool'"
	case TYPE_LIST:
		return "type 'list'"
	case TYPE_MAP:
		return "type 'map'"
	case TYPE_ANY:
		return "type 'any'"
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
	"schema":    SCHEMA,
	"str":       TYPE_STR,
	"int":       TYPE_INT,
	"float":     TYPE_FLOAT,
	"bool":      TYPE_BOOL,
	"list":      TYPE_LIST,
	"map":       TYPE_MAP,
	"any":       TYPE_ANY,
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
	case MODEL, AGENT, TASK, KNOWLEDGE, TRIGGER, WORKFLOW, TOOL, SCHEMA:
		return true
	}
	return false
}

// BlockName returns the lowercase block type name for a block keyword token
// (e.g. MODEL → "model"). Used for schema lookups. Returns empty string
// for non-block tokens.
func BlockName(t TokenType) string {
	if IsBlockKeyword(t) {
		return strings.ToLower(string(t))
	}
	return ""
}

// IsTypeAnnotation returns true if the token type is a type annotation
// keyword (str, int, float, bool, list, map, any).
func IsTypeAnnotation(t TokenType) bool {
	switch t {
	case TYPE_STR, TYPE_INT, TYPE_FLOAT, TYPE_BOOL, TYPE_LIST, TYPE_MAP, TYPE_ANY:
		return true
	}
	return false
}

// IsIdentLike returns true if the token type can serve as an identifier
// in contexts like assignment keys. Block keywords (model, agent, etc.)
// and type annotations (str, int, etc.) are valid key names inside
// blocks — e.g., `model = gpt4` inside an agent block uses "model"
// as a key, and `type = str` uses "str" as a value.
func IsIdentLike(t TokenType) bool {
	return t == IDENT || IsBlockKeyword(t) || IsTypeAnnotation(t)
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
	case DOT, LBRACKET, LPAREN:
		return PrecAccess
	default:
		return PrecLowest
	}
}

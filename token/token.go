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

// Token represents a single lexical token with its type, literal text,
// and source position. Line and Column enable source mapping from generated
// code back to the original .oc file.
type Token struct {
	Type    TokenType
	Literal string
	Line    int // 1-based line number in source
	Column  int // 1-based column number in source
}

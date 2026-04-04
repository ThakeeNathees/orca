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
	INT       TokenType = "INT"       // integer literal: 123
	FLOAT     TokenType = "FLOAT"     // float literal: 0.2
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

	// Boolean and null literals
	TRUE  TokenType = "TRUE"
	FALSE TokenType = "FALSE"
	NULL  TokenType = "NULL"

	// Keywords — each corresponds to a top-level block type in Orca syntax.
	MODEL     TokenType = "MODEL"
	AGENT     TokenType = "AGENT"
	KNOWLEDGE TokenType = "KNOWLEDGE"
	WORKFLOW  TokenType = "WORKFLOW"
	TOOL      TokenType = "TOOL"
	INPUT     TokenType = "INPUT"
	SCHEMA    TokenType = "SCHEMA"
	LET       TokenType = "LET"
	CRON      TokenType = "CRON"
	WEBHOOK   TokenType = "WEBHOOK"
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

// BlockKind identifies the kind of a top-level block.
// Primitive types (str, int, float, bool, any, null) are NOT block kinds —
// they are schemas defined in builtins.oc and represented as SchemaType.
type BlockKind int

const (
	BlockModel     BlockKind = iota // model block
	BlockAgent                      // agent block
	BlockTool                       // tool block
	BlockKnowledge                  // knowledge block
	BlockWorkflow                   // workflow block
	BlockInput                      // input block
	BlockSchema                     // schema block / user-defined schema types
	BlockLet                        // let block
	BlockCron                       // cron trigger block (workflow node)
	BlockWebhook                    // webhook trigger block (workflow node)
)

// blockKindStrings maps each BlockKind to its string representation.
// Indexed directly by BlockKind (contiguous iota values).
var blockKindStrings = [...]string{
	BlockModel: "model", BlockAgent: "agent", BlockTool: "tool",
	BlockKnowledge: "knowledge", BlockWorkflow: "workflow",
	BlockInput: "input", BlockSchema: "schema", BlockLet: "let",
	BlockCron: "cron", BlockWebhook: "webhook",
}

// String returns the string representation of a BlockKind.
func (k BlockKind) String() string {
	if int(k) >= 0 && int(k) < len(blockKindStrings) {
		return blockKindStrings[k]
	}
	return "unknown"
}

// BlockKinds lists all block kinds (not primitives).
var BlockKinds = []BlockKind{
	BlockModel, BlockAgent, BlockTool, BlockKnowledge,
	BlockWorkflow, BlockInput, BlockSchema, BlockLet,
	BlockCron, BlockWebhook,
}

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
	case INT:
		return "integer"
	case FLOAT:
		return "number"
	case STRING:
		return "string"
	case RAWSTRING:
		return "raw string"
	case TRUE, FALSE:
		return "boolean"
	case NULL:
		return "null"
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
	default:
		// Keywords like MODEL, AGENT, etc.
		if IsTokenBlockName(t) {
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
	"null":      NULL,
	"model":     MODEL,
	"agent":     AGENT,
	"knowledge": KNOWLEDGE,
	"workflow":  WORKFLOW,
	"tool":      TOOL,
	"input":     INPUT,
	"schema":    SCHEMA,
	"let":       LET,
	"cron":      CRON,
	"webhook":   WEBHOOK,
}

// LookupIdent checks if an identifier string is a reserved keyword.
// Returns the keyword's TokenType if found, otherwise returns IDENT.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}

// IsTokenBlockName returns true if the token type introduces a block
// (model, agent, tool, knowledge, workflow, input, schema, let, cron, webhook).
func IsTokenBlockName(t TokenType) bool {
	switch t {
	case MODEL, AGENT, KNOWLEDGE, WORKFLOW, TOOL, INPUT, SCHEMA, LET, CRON, WEBHOOK:
		return true
	}
	return false
}

// TokenTypeToBlockKind returns the BlockKind for a block keyword token type.
func TokenTypeToBlockKind(t TokenType) (BlockKind, bool) {
	switch t {
	case MODEL:
		return BlockModel, true
	case AGENT:
		return BlockAgent, true
	case TOOL:
		return BlockTool, true
	case KNOWLEDGE:
		return BlockKnowledge, true
	case WORKFLOW:
		return BlockWorkflow, true
	case INPUT:
		return BlockInput, true
	case SCHEMA:
		return BlockSchema, true
	case LET:
		return BlockLet, true
	case CRON:
		return BlockCron, true
	case WEBHOOK:
		return BlockWebhook, true
	default:
		return 0, false
	}
}

// IsIdentLike returns true if the token type can serve as an identifier
// in contexts like assignment keys. Block keywords (model, agent, etc.)
// are valid key names inside blocks — e.g., `model = gpt4` inside an
// agent block uses "model" as a key.
func IsIdentLike(t TokenType) bool {
	return t == IDENT || t == NULL || IsTokenBlockName(t)
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

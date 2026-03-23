// Package ast defines the Abstract Syntax Tree node types for the Orca language.
// Each node represents a syntactic construct (blocks, assignments, literals)
// produced by the parser from the token stream.
package ast

import "github.com/thakee/orca/compiler/token"

// Node is the interface that all AST nodes implement.
// Every node carries a source range (Start/End) for error reporting,
// source mapping, and editor integration. For terminal nodes (single-token
// literals), start and end point to the same token. For non-terminals
// (blocks, lists, assignments), they span the full range.
type Node interface {
	Start() token.Token
	End() token.Token
}

// Statement nodes implement this interface. The statementNode() marker method
// distinguishes statements from expressions at the type level, preventing
// accidental mixing in slices or function signatures.
type Statement interface {
	Node
	statementNode()
}

// Expression nodes implement this interface. Expressions produce values
// (strings, numbers, references, lists) and appear on the right-hand side
// of assignments.
type Expression interface {
	Node
	expressionNode()
}

// BaseNode is embedded in every AST node to provide source range tracking.
// For terminal nodes, TokenStart and TokenEnd are the same token.
// For non-terminal nodes, they mark the first and last token of the construct.
type BaseNode struct {
	TokenStart token.Token
	TokenEnd   token.Token
}

// NewTerminal creates a BaseNode where start and end are the same token.
// Used for single-token terminal nodes (identifiers, literals).
func NewTerminal(tok token.Token) BaseNode {
	return BaseNode{TokenStart: tok, TokenEnd: tok}
}

// Start returns the first token of this node's source range.
func (n *BaseNode) Start() token.Token { return n.TokenStart }

// End returns the last token of this node's source range.
func (n *BaseNode) End() token.Token { return n.TokenEnd }

// Program is the root node of every AST. It holds all top-level statements
// (blocks) parsed from a single .oc source file.
type Program struct {
	BaseNode
	Statements []Statement
	HasErrors  bool // true if the source had parse errors; AST may be partial
}

// Annotation represents a decorator on a field or block: @name or @name(args...).
// For example, @desc("The LLM provider") or @sensitive.
type Annotation struct {
	BaseNode
	Name      string       // annotation name without the @
	Arguments []Expression // e.g. @desc("text") has one StringLiteral arg
}

// BlockStatement represents any top-level block in Orca syntax:
//
//	keyword name {
//	  key = value
//	}
//
// The block kind ("model", "agent", etc.) is derived from TokenStart.Type.
// The span covers from the keyword token through the closing brace.
type BlockStatement struct {
	BaseNode
	Name        string        // the user-given name identifier after the keyword
	NameToken   token.Token   // the name token, used for diagnostic ranges
	OpenBrace   token.Token   // the '{' token, used for diagnostic ranges
	Assignments []*Assignment // key = value pairs inside the block body
	Annotations []*Annotation // decorators before the block keyword (@sensitive, etc.)
}

func (b *BlockStatement) statementNode() {}

// Identifier represents an unquoted name that references another block.
// For example, in `model = gpt4`, "gpt4" is an Identifier that refers
// to a model block defined elsewhere. Terminal node — start == end.
type Identifier struct {
	BaseNode
	Value string
}

func (i *Identifier) expressionNode() {}

// StringLiteral represents a double-quoted string value like "openai".
// Terminal node — start == end.
type StringLiteral struct {
	BaseNode
	Value string // the string content without surrounding quotes
}

func (s *StringLiteral) expressionNode() {}

// IntegerLiteral represents a whole number value like 4096.
// Terminal node — start == end.
type IntegerLiteral struct {
	BaseNode
	Value int64
}

func (il *IntegerLiteral) expressionNode() {}

// FloatLiteral represents a decimal number value like 0.2.
// Terminal node — start == end.
type FloatLiteral struct {
	BaseNode
	Value float64
}

func (fl *FloatLiteral) expressionNode() {}

// BooleanLiteral represents true or false.
// Terminal node — start == end.
type BooleanLiteral struct {
	BaseNode
	Value bool
}

func (bl *BooleanLiteral) expressionNode() {}

// BinaryExpression represents a binary operation: left op right.
// Examples: `a + b`, `1 * 2`, `researcher -> writer`.
// BaseNode spans from the left operand's start to the right operand's end.
type BinaryExpression struct {
	BaseNode
	Left     Expression
	Operator token.Token // the operator token (+, -, *, /, ->)
	Right    Expression
}

func (be *BinaryExpression) expressionNode() {}

// MemberAccess represents a dot access expression: object.member.
// For example, `workflow.report_pipeline` or `a.b.c`.
// BaseNode spans from the object's start to the member identifier.
type MemberAccess struct {
	BaseNode
	Object Expression // the left-hand side expression
	Member string     // the member name (right of the dot)
}

func (ma *MemberAccess) expressionNode() {}

// Subscription represents an index access expression: object[index].
// For example, `tools[0]` or `matrix[i + 1]`.
// BaseNode spans from the object's start to the closing bracket.
type Subscription struct {
	BaseNode
	Object Expression // the left-hand side expression
	Index  Expression // the expression inside the brackets
}

func (s *Subscription) expressionNode() {}

// CallExpression represents a function call: callee(arg1, arg2, ...).
// For example, `retry(3)` or `fallback(backup_agent, "default")`.
// BaseNode spans from the callee's start to the closing parenthesis.
type CallExpression struct {
	BaseNode
	Callee    Expression   // the expression being called
	Arguments []Expression // the argument expressions
}

func (ce *CallExpression) expressionNode() {}

// MapEntry represents a single key: value pair inside a map literal.
type MapEntry struct {
	Key   Expression
	Value Expression
}

// MapLiteral represents a map of key-value pairs like {name: "alice", age: 30}.
// Keys can be identifiers or strings. Values can be any expression.
// BaseNode covers from '{' to '}'.
type MapLiteral struct {
	BaseNode
	Entries []MapEntry
}

func (ml *MapLiteral) expressionNode() {}

// ListLiteral represents a bracketed list of expressions like [web_search, gmail]
// or ["read", "write"]. BaseNode covers from '[' to ']'.
type ListLiteral struct {
	BaseNode
	Elements []Expression
}

func (ll *ListLiteral) expressionNode() {}

// Assignment represents a key = value pair inside a block body.
// For example: `provider = "openai"` or `tools = [web_search, gmail]`.
// BaseNode covers from the key identifier to the last token of the value.
type Assignment struct {
	BaseNode
	Name        string        // the key (left-hand side)
	Value       Expression    // the value (right-hand side)
	Annotations []*Annotation // decorators before the key (@desc("..."), etc.)
}

func (a *Assignment) statementNode() {}

// NullLiteral represents the null keyword.
// Terminal node — start == end.
type NullLiteral struct {
	BaseNode
}

func (nl *NullLiteral) expressionNode() {}

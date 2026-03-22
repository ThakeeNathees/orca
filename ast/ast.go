// Package ast defines the Abstract Syntax Tree node types for the Orca language.
// Each node represents a syntactic construct (blocks, assignments, literals)
// produced by the parser from the token stream.
package ast

import "github.com/thakee/orca/token"

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

// Start returns the first token of this node's source range.
func (n *BaseNode) Start() token.Token { return n.TokenStart }

// End returns the last token of this node's source range.
func (n *BaseNode) End() token.Token { return n.TokenEnd }

// Program is the root node of every AST. It holds all top-level statements
// (blocks) parsed from a single .oc source file.
type Program struct {
	BaseNode
	Statements []Statement
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
	Assignments []*Assignment // key = value pairs inside the block body
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
	Name  string     // the key (left-hand side)
	Value Expression // the value (right-hand side)
}

func (a *Assignment) statementNode() {}

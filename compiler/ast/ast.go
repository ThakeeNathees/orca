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
//
// When adding a new concrete Expression type, update codegen (e.g.
// langgraph.exprToSource) with a case for it; otherwise Python generation
// will panic at compile/codegen time.
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

// findBlockWithName returns the BlockStatement with the given name, or nil.
func (p *Program) FindBlockWithName(name string) *BlockStatement {
	for _, stmt := range p.Statements {
		block, ok := stmt.(*BlockStatement)
		if ok && block.Name == name {
			return block
		}
	}
	return nil
}

// Annotation represents a decorator on a field or block: @name or @name(args...).
// For example, @desc("The LLM provider") or @sensitive.
type Annotation struct {
	BaseNode
	Name      string       // annotation name without the @
	Arguments []Expression // e.g. @desc("text") has one StringLiteral arg
}

// BlockBody holds the shared content of any block — its kind, field
// assignments, and bare expressions. Both BlockStatement (top-level named
// blocks) and BlockExpression (inline anonymous blocks) embed this so
// that analyzer, codegen, and tooling can operate on a single type.
type BlockBody struct {
	Kind        string        // the block type (model, agent, tool, …)
	Assignments []*Assignment // key = value pairs inside the block body
	Expressions []Expression  // workflow edge expressions (A -> B -> C)
	SourceFile  string        // the .oc file this block was parsed from
}

// GetFieldExpression returns the right-hand expression for the first assignment
// whose key matches field. If there is no such assignment, ok is false.
func (b *BlockBody) GetFieldExpression(field string) (expr Expression, ok bool) {
	if b == nil {
		return nil, false
	}
	for _, a := range b.Assignments {
		if a != nil && a.Name == field {
			return a.Value, true
		}
	}
	return nil, false
}

// BlockStatement represents any top-level block in Orca syntax:
//
//	keyword name {
//	  key = value
//	}
//
// The span covers from the keyword token through the closing brace.
type BlockStatement struct {
	BaseNode
	BlockBody
	Name        string        // the user-given name identifier after the keyword
	NameToken   token.Token   // the name token, used for diagnostic ranges
	OpenBrace   token.Token   // the '{' token, used for diagnostic ranges
	Annotations []*Annotation // decorators before the block keyword (@sensitive, etc.)
}

func (b *BlockStatement) statementNode() {}

// GetFieldExpression forwards to BlockBody.GetFieldExpression with a nil-safe
// guard, since Go cannot promote methods through a nil outer struct.
func (b *BlockStatement) GetFieldExpression(field string) (Expression, bool) {
	if b == nil {
		return nil, false
	}
	return b.BlockBody.GetFieldExpression(field)
}

// Identifier represents an unquoted name that references another block.
// For example, in `model = gpt4`, "gpt4" is an Identifier that refers
// to a model block defined elsewhere. Terminal node — start == end.
type Identifier struct {
	BaseNode
	Value string
}

func (i *Identifier) expressionNode() {}

// StringLiteral represents a string value — either a double-quoted string
// like "openai" or a triple-backtick raw string like ```md ... ```.
// Terminal node — start == end.
type StringLiteral struct {
	BaseNode
	Value string // the string content without surrounding delimiters
	Lang  string // optional language tag for raw strings (e.g. "md", "py")
}

func (s *StringLiteral) expressionNode() {}

// NumberLiteral represents both ints and floats (internally float64).
// Terminal node — start == end.
type NumberLiteral struct {
	BaseNode
	Value float64
}

func (nl *NumberLiteral) expressionNode() {}

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
	Object Expression  // the left-hand side expression
	Dot    token.Token // the '.' token, used for cursor position detection
	Member string      // the member name (right of the dot)
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

// BlockExpression represents an inline block definition: model { provider = "openai" ... }.
// Used for anonymous block instances in expressions like `model = model { provider = "openai" }`
// and inline schemas like `output = schema { draft = str }`.
// Works for all block types except let. BaseNode covers from the block keyword to the closing '}'.
type BlockExpression struct {
	BaseNode
	BlockBody
}

func (be *BlockExpression) expressionNode() {}

// NullLiteral represents the null keyword.
// Terminal node — start == end.
type NullLiteral struct {
	BaseNode
}

func (nl *NullLiteral) expressionNode() {}

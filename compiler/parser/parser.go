// Package parser implements a recursive descent parser for the Orca language.
// It consumes tokens from the lexer and produces an AST (Abstract Syntax Tree).
// Uses Pratt parsing for expressions with operator precedence.
package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/token"
)

// Parser holds the state for parsing a token stream into an AST.
// It uses a two-token lookahead (curToken + peekToken) to make parsing
// decisions without backtracking. prevToken tracks the last consumed token,
// needed to record TokenEnd on AST nodes after advancing past a closing
// delimiter.
type Parser struct {
	l           *lexer.Lexer
	diagnostics []diagnostic.Diagnostic
	prevToken   token.Token // the previously consumed token, used for span ends
	curToken    token.Token // the token currently being examined
	peekToken   token.Token // the next token, used for lookahead decisions
}

// New creates a parser for the given lexer and primes it by reading
// two tokens so both curToken and peekToken are set before parsing begins.
// The lexer's SourceFile is propagated to all parsed block nodes.
func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l}
	p.nextToken()
	p.nextToken()
	return p
}

// Errors returns all parse errors as strings for backward compatibility.
// Prefer Diagnostics() for structured error information.
func (p *Parser) Errors() []string {
	var errs []string
	for _, d := range p.diagnostics {
		errs = append(errs, d.Error())
	}
	return errs
}

// Diagnostics returns all diagnostics accumulated during parsing.
func (p *Parser) Diagnostics() []diagnostic.Diagnostic {
	return p.diagnostics
}

// nextToken advances the parser by one token, shifting peekToken into
// curToken and reading a fresh token from the lexer into peekToken.
// Keeps prevToken pointing at the token we just moved off of.
func (p *Parser) nextToken() {
	p.prevToken = p.curToken
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

// addErrorAt records a parse error at a specific token's position.
func (p *Parser) addErrorAt(tok token.Token, msg string) {
	p.diagnostics = append(p.diagnostics, diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     diagnostic.CodeSyntax,
		Position: diagnostic.Position{
			Line:   tok.Line,
			Column: tok.Column,
		},
		Message: msg,
		Source:  "parser",
	})
}

// addError records a parse error at the current token's position.
func (p *Parser) addError(msg string) {
	p.addErrorAt(p.curToken, msg)
}

// --- program & statements ---

// ParseProgram is the entry point. It parses the entire token stream into
// a Program node containing all top-level block statements. On parse errors,
// it skips the offending token and continues to find as many errors as possible.
func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.TokenStart = p.curToken

	for p.curToken.Type != token.EOF {
		beforeLine, beforeCol := p.curToken.Line, p.curToken.Column
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
			continue
		}
		// If error recovery didn't advance, skip a token to prevent infinite loops.
		if p.curToken.Line == beforeLine && p.curToken.Column == beforeCol {
			p.nextToken()
		}
	}

	program.TokenEnd = p.curToken // EOF token
	program.HasErrors = len(p.diagnostics) > 0
	return program
}

// parseStatement dispatches to the appropriate block parser based on the
// current token type. Collects any leading annotations (@sensitive, etc.)
// and attaches them to the block. Returns nil with an error for unrecognized tokens.
func (p *Parser) parseStatement() ast.Statement {
	annotations := p.collectAnnotations()

	if p.curToken.Type == token.IDENT {
		block := p.parseBlock()
		if block == nil {
			return nil
		}
		block.Annotations = annotations
		return block
	}
	p.addError(fmt.Sprintf("expected block kind (identifier), got %s",
		token.Describe(p.curToken.Type)))
	return nil
}

// parseBlock parses a generic block: `keyword name { assignments }`.
// All block types (model, agent, tool, etc.) share this same structure,
// so a single method handles them all. The block's kind is determined
// by the keyword token type stored in TokenStart.
//
// Error-tolerant: returns a partial BlockStatement even when the body
// has syntax errors, so LSP features (completion, hover) still work
// on incomplete input.
func (p *Parser) parseBlock() *ast.BlockStatement {
	block := &ast.BlockStatement{}
	block.Kind = p.curToken.Literal
	block.SourceFile = p.l.SourceFile
	block.TokenStart = p.curToken

	// Expect the block's name identifier (e.g., "gpt4" in "model gpt4 {").
	if p.peekToken.Type != token.IDENT {
		p.addErrorAt(p.peekToken, fmt.Sprintf("expected name after '%s', got %s",
			block.Kind, token.Describe(p.peekToken.Type)))
		p.syncToBlockEnd()
		return nil
	}
	p.nextToken()
	block.Name = p.curToken.Literal
	block.NameToken = p.curToken

	// Expect the opening brace.
	if p.peekToken.Type != token.LBRACE {
		p.addErrorAt(p.peekToken, fmt.Sprintf("expected '{' after '%s %s', got %s",
			block.Kind, block.Name, token.Describe(p.peekToken.Type)))
		p.syncToBlockEnd()
		return nil
	}
	p.nextToken()
	block.OpenBrace = p.curToken

	// Parse the body: assignments and/or edge expressions (A -> B -> C).
	// The analyzer validates whether expressions are allowed for the block kind.
	block.Assignments, block.Expressions = p.parseBlockBody(block.Kind, block.Name)

	// Expect the closing brace.
	if p.curToken.Type != token.RBRACE {
		p.addError(fmt.Sprintf("expected '}' to close '%s %s' block, got %s",
			block.Kind, block.Name, token.Describe(p.curToken.Type)))
		// Return the partial block with whatever assignments parsed.
		block.TokenEnd = p.prevToken
		return block
	}
	block.TokenEnd = p.curToken // the } token
	p.nextToken()               // consume }

	return block
}

// parseAssignment parses a single `key = value` pair. The key must be an
// identifier or a keyword used as an identifier (like "model" inside a block).
//
// Error-tolerant: on failure, syncs to the next assignment boundary
// (next identifier-like token at the same nesting level, or '}'/EOF)
// so subsequent assignments can still be parsed.
func (p *Parser) parseAssignment(blockKind string, blockName string) *ast.Assignment {
	annotations := p.collectAnnotations()

	if p.curToken.Type != token.IDENT {
		p.addError(fmt.Sprintf("expected identifier for assignment key in %s %s, got %s",
			blockKind, blockName, token.Describe(p.curToken.Type)))
		p.syncToNextAssignment()
		return nil
	}

	a := &ast.Assignment{}
	a.Annotations = annotations
	a.TokenStart = p.curToken // the key identifier
	a.NameToken = p.curToken
	a.Name = p.curToken.Literal
	key := p.curToken.Literal

	// Expect = after the key.
	if p.peekToken.Type != token.ASSIGN {
		p.addErrorAt(p.peekToken, fmt.Sprintf("expected '=' after '%s', got %s",
			key, token.Describe(p.peekToken.Type)))
		p.syncToNextAssignment()
		return nil
	}
	p.nextToken() // move to =
	p.nextToken() // move past =

	// Parse the right-hand side expression with lowest precedence.
	val := p.parseExpression(token.PrecLowest)
	if val == nil {
		p.syncToNextAssignment()
		return nil
	}
	a.Value = val
	// The expression parser advances past the value, so prevToken holds
	// the last token of the value expression.
	a.TokenEnd = p.prevToken

	return a
}

// collectAnnotations parses zero or more consecutive annotations.
// Returns nil if no annotations are present (no allocation).
func (p *Parser) collectAnnotations() []*ast.Annotation {
	var annotations []*ast.Annotation
	for p.curToken.Type == token.AT {
		ann := p.parseAnnotation()
		if ann != nil {
			annotations = append(annotations, ann)
		}
	}
	return annotations
}

// parseAnnotation parses an annotation: @name or @name(args...).
// Assumes curToken is AT. Returns nil on error.
func (p *Parser) parseAnnotation() *ast.Annotation {
	ann := &ast.Annotation{}
	ann.TokenStart = p.curToken // the @ token
	p.nextToken()               // move past @

	if p.curToken.Type != token.IDENT {
		p.addError(fmt.Sprintf("expected annotation name after '@', got %s",
			token.Describe(p.curToken.Type)))
		return nil
	}
	ann.Name = p.curToken.Literal
	ann.TokenEnd = p.curToken
	p.nextToken() // move past name

	// Check for optional argument list: @name(args...)
	if p.curToken.Type == token.LPAREN {
		p.nextToken() // move past (
		for p.curToken.Type != token.RPAREN && p.curToken.Type != token.EOF {
			arg := p.parseExpression(token.PrecLowest)
			if arg == nil {
				return nil
			}
			ann.Arguments = append(ann.Arguments, arg)
			if p.curToken.Type == token.COMMA {
				p.nextToken() // consume comma
			}
		}
		if p.curToken.Type != token.RPAREN {
			p.addError("expected ')' to close annotation arguments")
			return nil
		}
		ann.TokenEnd = p.curToken
		p.nextToken() // move past )
	}

	return ann
}

// --- error recovery ---

// syncToBlockEnd advances past tokens until it reaches a '}', a block keyword,
// or EOF. Used when a block header fails to parse (missing name or '{').
func (p *Parser) syncToBlockEnd() {
	for p.curToken.Type != token.EOF {
		if p.curToken.Type == token.RBRACE {
			p.nextToken() // consume the }
			return
		}
		if p.curToken.Type == token.IDENT {
			return // stop before the keyword so the main loop can parse it
		}
		p.nextToken()
	}
}

// syncToNextAssignment advances past tokens until it finds a position where
// a new assignment could start: an identifier-like token followed by '=',
// a '}', or EOF. Always advances at least one token to avoid infinite loops.
func (p *Parser) syncToNextAssignment() {
	p.nextToken() // always advance at least once
	for p.curToken.Type != token.EOF && p.curToken.Type != token.RBRACE {
		if p.curToken.Type == token.IDENT && p.peekToken.Type == token.ASSIGN {
			return
		}
		p.nextToken()
	}
}

// --- expressions (Pratt parser) ---

// parseExpression implements Pratt parsing. It parses a primary (atom) on the
// left, then repeatedly consumes binary operators whose precedence is higher
// than the caller's level, building a left-associative BinaryExpression tree.
func (p *Parser) parseExpression(precedence int) ast.Expression {
	left := p.parsePrimary()
	if left == nil {
		return nil
	}

	// While the next token is a binary operator with higher precedence
	// than our current level, consume it and parse the right-hand side.
	for token.Precedence(p.curToken.Type) > precedence {
		if p.curToken.Type == token.DOT {
			left = p.parseMemberAccess(left)
			continue
		}

		if p.curToken.Type == token.LBRACKET {
			left = p.parseSubscription(left)
			continue
		}

		if p.curToken.Type == token.LPAREN {
			left = p.parseCallExpression(left)
			continue
		}

		if p.curToken.Type == token.QUESTION {
			left = p.parseTernaryExpression(left)
			continue
		}

		op := p.curToken
		prec := token.Precedence(op.Type)
		p.nextToken() // consume the operator

		right := p.parseExpression(prec)
		if right == nil {
			return nil
		}

		left = &ast.BinaryExpression{
			BaseNode: ast.BaseNode{
				TokenStart: left.Start(),
				TokenEnd:   right.End(),
			},
			Left:     left,
			Operator: op,
			Right:    right,
		}
	}

	return left
}

// parsePrimary parses an atomic expression: literals, identifiers, and lists.
// These are the leaves of the expression tree that binary operators combine.
func (p *Parser) parsePrimary() ast.Expression {
	switch p.curToken.Type {
	case token.STRING:
		expr := &ast.StringLiteral{BaseNode: ast.NewTerminal(p.curToken), Value: p.curToken.Literal}
		p.nextToken()
		return expr

	case token.RAWSTRING:
		// Raw string literal is "lang\ncontent" — split on first newline.
		lang, content, _ := strings.Cut(p.curToken.Literal, "\n")
		expr := &ast.StringLiteral{BaseNode: ast.NewTerminal(p.curToken), Value: content, Lang: lang}
		p.nextToken()
		return expr

	case token.NUMBER:
		// Parse all numbers as float64 (single NumberLiteral type)
		if val, err := strconv.ParseFloat(p.curToken.Literal, 64); err == nil {
			expr := &ast.NumberLiteral{BaseNode: ast.NewTerminal(p.curToken), Value: val}
			p.nextToken()
			return expr
		} else {
			p.addError(fmt.Sprintf("invalid number literal '%s'", p.curToken.Literal))
			return nil
		}

	// In Orca's philosophy there is no special identifiers (example true, false, null, schema, etc.).
	// However orca has them, so null and bool are schemas defined in the bootstrap.oc and
	// true/false are "instances" of bool, and they defined as such:
	//
	//     bool true {}
	//     bool false {}
	//
	// Also bool is not a special keyword, it's another identifier defined as such:
	//
	//    schema bool {}
	//
	// And you've guessed it, schema is an identifier as well, where the schema
	// is bootstrapped as
	//
	//    schema schema {}
	//
	case token.IDENT:
		// If followed by '{', parse as inline block expression.
		// Otherwise treat as identifier (e.g., model = gpt4 inside an agent block).
		if p.peekToken.Type == token.LBRACE {
			return p.parseBlockExpression()
		}
		expr := &ast.Identifier{BaseNode: ast.NewTerminal(p.curToken), Value: p.curToken.Literal}
		p.nextToken()
		return expr

	case token.LBRACKET:
		return p.parseList()

	case token.LBRACE:
		return p.parseMap()

	case token.BACKSLASH:
		return p.parseLambda()

	default:
		p.addError(fmt.Sprintf("expected value, got %s", token.Describe(p.curToken.Type)))
		return nil
	}
}

// parseLambda parses a lambda expression: \(params) return_type -> body.
// The '\' token must be the current token.
func (p *Parser) parseLambda() ast.Expression {
	startToken := p.curToken
	p.nextToken() // consume '\'

	if p.curToken.Type != token.LPAREN {
		p.addError(fmt.Sprintf("expected '(' after '\\', got %s", token.Describe(p.curToken.Type)))
		return nil
	}
	p.nextToken() // consume '('

	// Parse parameters: name type, name type, ...
	var params []ast.LambdaParam
	for p.curToken.Type != token.RPAREN && p.curToken.Type != token.EOF {
		if p.curToken.Type != token.IDENT {
			p.addError(fmt.Sprintf("expected parameter name, got %s", token.Describe(p.curToken.Type)))
			return nil
		}
		nameIdent := &ast.Identifier{BaseNode: ast.NewTerminal(p.curToken), Value: p.curToken.Literal}
		p.nextToken() // consume param name

		// Parse the type expression for this parameter.
		typeExpr := p.parseExpression(token.PrecLowest)
		if typeExpr == nil {
			return nil
		}

		params = append(params, ast.LambdaParam{Name: nameIdent, TypeExpr: typeExpr})

		if p.curToken.Type == token.COMMA {
			p.nextToken() // consume ','
		}
	}

	if p.curToken.Type != token.RPAREN {
		p.addError(fmt.Sprintf("expected ')' to close lambda params, got %s", token.Describe(p.curToken.Type)))
		return nil
	}
	p.nextToken() // consume ')'

	// Parse optional return type: everything before '->' is the return type.
	// If the next token is '->', there's no return type.
	// Parse at PrecArrow so '->' stops the expression.
	var returnType ast.Expression
	if p.curToken.Type != token.ARROW {
		returnType = p.parseExpression(token.PrecArrow)
		if returnType == nil {
			return nil
		}
	}

	if p.curToken.Type != token.ARROW {
		p.addError(fmt.Sprintf("expected '->' in lambda expression, got %s", token.Describe(p.curToken.Type)))
		return nil
	}
	arrowToken := p.curToken
	p.nextToken() // consume '->'

	body := p.parseExpression(token.PrecLowest)
	if body == nil {
		return nil
	}

	return &ast.Lambda{
		BaseNode: ast.BaseNode{
			TokenStart: startToken,
			TokenEnd:   body.End(),
		},
		Params:     params,
		ReturnType: returnType,
		Arrow:      arrowToken,
		Body:       body,
	}
}

// parseTernaryExpression parses a ternary conditional: condition ? trueExpr : falseExpr.
// The condition has already been parsed as 'left'. The '?' must be the current token.
// Right-associative: a ? b ? c : d : e parses as a ? (b ? c : d) : e.
func (p *Parser) parseTernaryExpression(condition ast.Expression) ast.Expression {
	question := p.curToken
	p.nextToken() // consume ?

	// Parse true branch — use PrecLowest so nested ternaries bind right.
	trueExpr := p.parseExpression(token.PrecLowest)
	if trueExpr == nil {
		return nil
	}

	// Expect colon separator.
	if p.curToken.Type != token.COLON {
		p.addError(fmt.Sprintf("expected ':' in ternary expression, got %s",
			token.Describe(p.curToken.Type)))
		return nil
	}
	colon := p.curToken
	p.nextToken() // consume :

	// Parse false branch — use PrecLowest so nested ternaries bind right.
	falseExpr := p.parseExpression(token.PrecLowest)
	if falseExpr == nil {
		p.addError("expected expression after ':' in ternary")
		return nil
	}

	return &ast.TernaryExpression{
		BaseNode: ast.BaseNode{
			TokenStart: condition.Start(),
			TokenEnd:   falseExpr.End(),
		},
		Condition: condition,
		Question:  question,
		TrueExpr:  trueExpr,
		Colon:     colon,
		FalseExpr: falseExpr,
	}
}

// parseMemberAccess parses a dot access: object.member.
// The dot token must be the current token. The right side must be an identifier.
func (p *Parser) parseMemberAccess(object ast.Expression) *ast.MemberAccess {
	dotToken := p.curToken
	p.nextToken() // consume the dot

	if p.curToken.Type != token.IDENT {
		p.addError(fmt.Sprintf("expected member name after '.', got %s",
			token.Describe(p.curToken.Type)))
		// Return a partial MemberAccess with empty Member so the AST is
		// never nil. This allows the analyzer and LSP to work with
		// incomplete expressions (e.g. "gpt4." while the user is typing).
		return &ast.MemberAccess{
			BaseNode: ast.BaseNode{
				TokenStart: object.Start(),
				TokenEnd:   dotToken,
			},
			Object: object,
			Dot:    dotToken,
			Member: "",
		}
	}

	ma := &ast.MemberAccess{
		BaseNode: ast.BaseNode{
			TokenStart: object.Start(),
			TokenEnd:   p.curToken,
		},
		Object: object,
		Dot:    dotToken,
		Member: p.curToken.Literal,
	}
	p.nextToken() // consume the member identifier
	return ma
}

// parseSubscription parses an index access: object[index] or object[a, b, c].
// The '[' token must be the current token. Indices are comma-separated expressions.
func (p *Parser) parseSubscription(object ast.Expression) *ast.Subscription {
	openBracket := p.curToken
	p.nextToken() // consume the [

	var indices []ast.Expression
	first := p.parseExpression(token.PrecLowest)
	if first == nil {
		// Return a partial Subscription with empty Indices so the AST is never
		// nil. The analyzer will skip validation for nil sub-expressions.
		return &ast.Subscription{
			BaseNode: ast.BaseNode{
				TokenStart: object.Start(),
				TokenEnd:   openBracket,
			},
			Object: object,
		}
	}
	indices = append(indices, first)

	// Parse additional comma-separated indices.
	for p.curToken.Type == token.COMMA {
		p.nextToken() // consume the comma
		idx := p.parseExpression(token.PrecLowest)
		if idx == nil {
			break
		}
		indices = append(indices, idx)
	}

	if p.curToken.Type != token.RBRACKET {
		p.addError(fmt.Sprintf("expected ']' to close subscript, got %s",
			token.Describe(p.curToken.Type)))
		// Return partial with what we have so far.
		last := indices[len(indices)-1]
		return &ast.Subscription{
			BaseNode: ast.BaseNode{
				TokenStart: object.Start(),
				TokenEnd:   last.End(),
			},
			Object:  object,
			Indices: indices,
		}
	}

	sub := &ast.Subscription{
		BaseNode: ast.BaseNode{
			TokenStart: object.Start(),
			TokenEnd:   p.curToken,
		},
		Object:  object,
		Indices: indices,
	}
	p.nextToken() // consume the ]
	return sub
}

// parseCallExpression parses a function call: callee(arg1, arg2, ...).
// The '(' token must be the current token. Arguments are comma-separated expressions.
func (p *Parser) parseCallExpression(callee ast.Expression) *ast.CallExpression {
	call := &ast.CallExpression{
		Callee: callee,
	}
	call.TokenStart = callee.Start()
	p.nextToken() // consume the (

	// Handle empty argument list.
	if p.curToken.Type == token.RPAREN {
		call.TokenEnd = p.curToken
		p.nextToken() // consume )
		return call
	}

	// Parse the first argument.
	arg := p.parseExpression(token.PrecLowest)
	if arg != nil {
		call.Arguments = append(call.Arguments, arg)
	}

	// Parse remaining comma-separated arguments.
	for p.curToken.Type == token.COMMA {
		p.nextToken() // move past ,
		arg := p.parseExpression(token.PrecLowest)
		if arg != nil {
			call.Arguments = append(call.Arguments, arg)
		}
	}

	// Expect closing parenthesis.
	if p.curToken.Type != token.RPAREN {
		p.addError(fmt.Sprintf("expected ')' to close function call, got %s",
			token.Describe(p.curToken.Type)))
		// Return partial call so the AST is never a typed nil.
		call.TokenEnd = callee.End()
		return call
	}
	call.TokenEnd = p.curToken
	p.nextToken() // consume )

	return call
}

// parseList parses a bracketed list expression: [elem, elem, ...].
// Elements can be any expression type (including binary expressions).
// Handles empty lists, trailing commas are not supported.
func (p *Parser) parseList() ast.Expression {
	list := &ast.ListLiteral{}
	list.TokenStart = p.curToken // the [ token
	p.nextToken()                // move past [

	// Handle empty list [].
	if p.curToken.Type == token.RBRACKET {
		list.TokenEnd = p.curToken // the ] token
		p.nextToken()
		return list
	}

	// Parse the first element.
	elem := p.parseExpression(token.PrecLowest)
	if elem == nil {
		return nil
	}
	list.Elements = append(list.Elements, elem)

	// Parse remaining comma-separated elements.
	for p.curToken.Type == token.COMMA {
		p.nextToken() // move past ,
		// Allow trailing comma: if we see ] after comma, stop.
		if p.curToken.Type == token.RBRACKET {
			break
		}
		elem := p.parseExpression(token.PrecLowest)
		if elem == nil {
			return nil
		}
		list.Elements = append(list.Elements, elem)
	}

	// Expect closing bracket.
	if p.curToken.Type != token.RBRACKET {
		p.addError(fmt.Sprintf("expected ']' to close list, got %s",
			token.Describe(p.curToken.Type)))
		return nil
	}
	list.TokenEnd = p.curToken // the ] token
	p.nextToken()              // move past ]

	return list
}

// parseBlockExpression parses an inline block definition: model { key = value ... }.
// The block keyword must be the current token with `{` as the peek token.
// Works for all block types except let.
func (p *Parser) parseBlockExpression() *ast.BlockExpression {
	be := &ast.BlockExpression{}
	be.Kind = p.curToken.Literal
	be.SourceFile = p.l.SourceFile
	be.TokenStart = p.curToken // the block keyword

	p.nextToken() // move to {
	be.Assignments, be.Expressions = p.parseBlockBody(be.Kind, "inline")

	if p.curToken.Type != token.RBRACE {
		p.addError(fmt.Sprintf("expected '}' to close inline %s, got %s",
			be.Kind, token.Describe(p.curToken.Type)))
		return nil
	}
	be.TokenEnd = p.curToken
	p.nextToken() // consume }
	return be
}

// parseBlockBody parses a block body that can contain both key = value
// assignments and bare expressions (e.g. edge chains A -> B -> C).
// An identifier followed by '=' is parsed as an assignment; otherwise it's
// parsed as an expression. The analyzer validates which block kinds allow expressions.
func (p *Parser) parseBlockBody(blockKind, blockName string) ([]*ast.Assignment, []ast.Expression) {
	var assignments []*ast.Assignment
	var expressions []ast.Expression
	p.nextToken() // move past {

	for p.curToken.Type != token.RBRACE && p.curToken.Type != token.EOF {
		// Annotations or identifier followed by '=' indicate an assignment.
		if p.curToken.Type == token.AT ||
			(p.curToken.Type == token.IDENT && p.peekToken.Type == token.ASSIGN) {
			a := p.parseAssignment(blockKind, blockName)
			if a != nil {
				assignments = append(assignments, a)
			}
			continue
		}

		// Otherwise parse as a bare expression (e.g. edge chain A -> B -> C).
		expr := p.parseExpression(token.PrecLowest)
		if expr == nil {
			p.syncToNextAssignment()
			continue
		}
		expressions = append(expressions, expr)
	}

	return assignments, expressions
}

// parseMap parses a map literal: {key: value, key: value, ...}.
// Keys can be identifiers or strings. Values can be any expression.
func (p *Parser) parseMap() ast.Expression {
	m := &ast.MapLiteral{}
	m.TokenStart = p.curToken // the { token
	p.nextToken()             // move past {

	// Handle empty map {}.
	if p.curToken.Type == token.RBRACE {
		m.TokenEnd = p.curToken
		p.nextToken()
		return m
	}

	// Parse the first entry.
	entry, ok := p.parseMapEntry()
	if !ok {
		return nil
	}
	m.Entries = append(m.Entries, entry)

	// Parse remaining comma-separated entries.
	for p.curToken.Type == token.COMMA {
		p.nextToken() // move past ,
		// Allow trailing comma: if we see } after comma, stop.
		if p.curToken.Type == token.RBRACE {
			break
		}
		entry, ok := p.parseMapEntry()
		if !ok {
			return nil
		}
		m.Entries = append(m.Entries, entry)
	}

	// Expect closing brace.
	if p.curToken.Type != token.RBRACE {
		p.addError(fmt.Sprintf("expected '}' to close map, got %s",
			token.Describe(p.curToken.Type)))
		return nil
	}
	m.TokenEnd = p.curToken
	p.nextToken() // move past }

	return m
}

// parseMapEntry parses a single key: value pair inside a map literal.
func (p *Parser) parseMapEntry() (ast.MapEntry, bool) {
	key := p.parseExpression(token.PrecLowest)
	if key == nil {
		return ast.MapEntry{}, false
	}

	if p.curToken.Type != token.COLON {
		p.addError(fmt.Sprintf("expected ':' after map key, got %s",
			token.Describe(p.curToken.Type)))
		return ast.MapEntry{}, false
	}
	p.nextToken() // consume :

	value := p.parseExpression(token.PrecLowest)
	if value == nil {
		return ast.MapEntry{}, false
	}

	return ast.MapEntry{Key: key, Value: value}, true
}

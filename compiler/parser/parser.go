// Package parser implements a recursive descent parser for the Orca language.
// It consumes tokens from the lexer and produces an AST (Abstract Syntax Tree).
// Uses Pratt parsing for expressions with operator precedence.
package parser

import (
	"fmt"
	"strconv"

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

// addError records a parse error at the current token's position.
func (p *Parser) addError(msg string) {
	p.diagnostics = append(p.diagnostics, diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Position: diagnostic.Position{
			Line:   p.curToken.Line,
			Column: p.curToken.Column,
		},
		Message: msg,
		Source:  "parser",
	})
}

// expectPeek checks if the next token matches the expected type.
// If it matches, it advances the parser and returns true.
// If not, it records an error, advances anyway (to prevent infinite loops
// on malformed input), and returns false.
func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekToken.Type == t {
		p.nextToken()
		return true
	}
	p.addError(fmt.Sprintf("expected %s, got %s", t, p.peekToken.Type))
	p.nextToken() // advance past the unexpected token
	return false
}

// --- program & statements ---

// ParseProgram is the entry point. It parses the entire token stream into
// a Program node containing all top-level block statements. On parse errors,
// it skips the offending token and continues to find as many errors as possible.
func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.TokenStart = p.curToken

	for p.curToken.Type != token.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		} else {
			// Skip the current token to avoid infinite loops when we
			// encounter something we can't parse.
			p.nextToken()
		}
	}

	program.TokenEnd = p.curToken // EOF token
	return program
}

// parseStatement dispatches to the appropriate block parser based on the
// current token type. Returns nil with an error for unrecognized tokens.
func (p *Parser) parseStatement() ast.Statement {
	if token.IsBlockKeyword(p.curToken.Type) {
		return p.parseBlock()
	}
	p.addError(fmt.Sprintf("unexpected token %s", p.curToken.Type))
	return nil
}

// parseBlock parses a generic block: `keyword name { assignments }`.
// All block types (model, agent, tool, etc.) share this same structure,
// so a single method handles them all. The block's kind is determined
// by the keyword token type stored in TokenStart.
func (p *Parser) parseBlock() *ast.BlockStatement {
	block := &ast.BlockStatement{}
	block.TokenStart = p.curToken // the keyword token (MODEL, AGENT, etc.)

	// Expect the block's name identifier (e.g., "gpt4" in "model gpt4 {").
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	block.Name = p.curToken.Literal

	// Expect the opening brace.
	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	// Parse the body: zero or more key = value assignments.
	block.Assignments = p.parseAssignments()

	// Expect the closing brace.
	if p.curToken.Type != token.RBRACE {
		p.addError("expected }")
		return nil
	}
	block.TokenEnd = p.curToken // the } token
	p.nextToken()               // consume }

	return block
}

// parseAssignments parses all key = value pairs inside a block body,
// stopping when it hits a closing brace or EOF. Returns the collected
// assignments as a slice.
func (p *Parser) parseAssignments() []*ast.Assignment {
	var assignments []*ast.Assignment
	p.nextToken() // move past {

	for p.curToken.Type != token.RBRACE && p.curToken.Type != token.EOF {
		a := p.parseAssignment()
		if a != nil {
			assignments = append(assignments, a)
		} else {
			// Skip on error to avoid getting stuck.
			p.nextToken()
		}
	}

	return assignments
}

// parseAssignment parses a single `key = value` pair. The key must be an
// identifier or a keyword used as an identifier (like "model" inside a block).
func (p *Parser) parseAssignment() *ast.Assignment {
	if !token.IsIdentLike(p.curToken.Type) {
		p.addError(fmt.Sprintf("expected identifier, got %s", p.curToken.Type))
		return nil
	}

	a := &ast.Assignment{}
	a.TokenStart = p.curToken // the key identifier
	a.Name = p.curToken.Literal

	// Expect = after the key.
	if !p.expectPeek(token.ASSIGN) {
		return nil
	}
	p.nextToken() // move past =

	// Parse the right-hand side expression with lowest precedence.
	val := p.parseExpression(token.PrecLowest)
	if val == nil {
		return nil
	}
	a.Value = val
	// The expression parser advances past the value, so prevToken holds
	// the last token of the value expression.
	a.TokenEnd = p.prevToken

	return a
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

	case token.INT:
		val, err := strconv.ParseInt(p.curToken.Literal, 10, 64)
		if err != nil {
			p.addError(fmt.Sprintf("could not parse %q as integer", p.curToken.Literal))
			return nil
		}
		expr := &ast.IntegerLiteral{BaseNode: ast.NewTerminal(p.curToken), Value: val}
		p.nextToken()
		return expr

	case token.FLOAT:
		val, err := strconv.ParseFloat(p.curToken.Literal, 64)
		if err != nil {
			p.addError(fmt.Sprintf("could not parse %q as float", p.curToken.Literal))
			return nil
		}
		expr := &ast.FloatLiteral{BaseNode: ast.NewTerminal(p.curToken), Value: val}
		p.nextToken()
		return expr

	case token.TRUE:
		expr := &ast.BooleanLiteral{BaseNode: ast.NewTerminal(p.curToken), Value: true}
		p.nextToken()
		return expr

	case token.FALSE:
		expr := &ast.BooleanLiteral{BaseNode: ast.NewTerminal(p.curToken), Value: false}
		p.nextToken()
		return expr

	case token.IDENT:
		// An unquoted identifier is a reference to another block.
		expr := &ast.Identifier{BaseNode: ast.NewTerminal(p.curToken), Value: p.curToken.Literal}
		p.nextToken()
		return expr

	case token.LBRACKET:
		return p.parseList()

	default:
		p.addError(fmt.Sprintf("unexpected value token %s", p.curToken.Type))
		return nil
	}
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
		elem := p.parseExpression(token.PrecLowest)
		if elem == nil {
			return nil
		}
		list.Elements = append(list.Elements, elem)
	}

	// Expect closing bracket.
	if p.curToken.Type != token.RBRACKET {
		p.addError("expected ]")
		return nil
	}
	list.TokenEnd = p.curToken // the ] token
	p.nextToken()              // move past ]

	return list
}

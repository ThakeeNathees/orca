// Package diagnostic defines the error/warning types used across the
// Orca compiler pipeline. Both the parser and analyzer produce Diagnostics,
// and the LSP server converts them to LSP protocol format for editors.
package diagnostic

import (
	"fmt"

	"github.com/thakee/orca/orca/compiler/ast"
	"github.com/thakee/orca/orca/compiler/token"
)

// Severity indicates how serious a diagnostic is.
type Severity int

const (
	Error   Severity = iota // a problem that prevents compilation
	Warning                 // a potential issue that doesn't block compilation
	Info                    // informational message
	Hint                    // a suggestion for improvement
)

// Position represents a source location in an .orca file.
// Line and Column are 1-based, matching the lexer's convention.
// File is the origin .orca path, propagated from the token's SourceFile;
// it's empty for in-memory/test inputs.
type Position struct {
	Line   int
	Column int
	File   string
}

// Diagnostic codes identify each kind of diagnostic for suppression
// with @suppress("code") and display in the CLI/editor.
const (
	CodeSyntax               = "syntax"                 // parse errors
	CodeDuplicateBlock       = "duplicate-block"        // duplicate block name
	CodeDuplicateField       = "duplicate-field"        // duplicate field in block
	CodeMissingField         = "missing-field"          // required field not present
	CodeUnknownField         = "unknown-field"          // field not defined in schema
	CodeTypeMismatch         = "type-mismatch"          // field value type doesn't match schema
	CodeUndefinedRef         = "undefined-ref"          // identifier not in symbol table
	CodeUnknownMember        = "unknown-member"         // member not found on block type
	CodeInvalidSubscript     = "invalid-subscript"      // non-integer subscript on a list
	CodeInvalidValue         = "invalid-value"          // field value not in allowed set
	CodeUnknownProvider      = "unknown-provider"       // model provider not supported by backend
	CodeUnsupportedLang      = "unsupported-lang"       // raw string language not supported by backend
	CodeUnexpectedExpr       = "unexpected-expr"        // expression not allowed in this context
	CodeInvalidWorkNode      = "invalid-workflow-node"  // block kind not allowed as workflow node
	CodeTriggerAsTarget      = "trigger-as-target"      // trigger block used as edge target instead of source
	CodeCyclicDependency     = "cyclic-dependency"      // blocks form a dependency cycle
	CodeInvalidArgumentCount = "invalid-argument-count" // lambda expects wrong number of arguments
)

// Diagnostic represents a single compiler message (error, warning, etc.)
// tied to a source location. Used by the parser, analyzer, and codegen
// stages, then converted to LSP diagnostics for editor integration.
type Diagnostic struct {
	Severity    Severity
	Code        string   // unique identifier for this diagnostic kind (e.g. "undefined-ref")
	Position    Position // start of the diagnostic range
	EndPosition Position // end of the diagnostic range (zero value means same as Position)
	Message     string
	Source      string // which stage produced this: "parser", "analyzer", etc.
}

// Error implements the error interface so a Diagnostic can be used as a Go error.
// Prefers the origin file for the location prefix, falling back to the
// producing stage (Source) only when no file is known.
func (d Diagnostic) Error() string {
	loc := d.Position.File
	if loc == "" {
		loc = d.Source
	}
	return fmt.Sprintf("%s:%d:%d: [%s] %s", loc, d.Position.Line, d.Position.Column, d.Code, d.Message)
}

// PositionOf returns the start Position of a token.
func PositionOf(t token.Token) Position {
	return Position{Line: t.Line, Column: t.Column, File: t.SourceFile}
}

// EndPositionOf returns the exclusive end Position of a token — the column
// just past its last character. The lexer stores EndLine/EndCol as the
// *inclusive* position of the token's last character, so this adds 1 to
// produce the half-open range convention used by Diagnostic.EndPosition.
// Falls back to Column + len(Literal) for tokens that didn't populate the
// end fields.
func EndPositionOf(t token.Token) Position {
	if t.EndLine != 0 {
		return Position{Line: t.EndLine, Column: t.EndCol + 1, File: t.SourceFile}
	}
	if t.EndCol != 0 {
		return Position{Line: t.Line, Column: t.EndCol + 1, File: t.SourceFile}
	}
	return Position{Line: t.Line, Column: t.Column + len(t.Literal), File: t.SourceFile}
}

// RangeOf returns the (start, end) positions covering the full source
// range of an AST node, derived from its Start/End tokens. Use this when
// building a Diagnostic from a node so EndPosition is always filled in —
// previously many call sites built Position manually and left EndPosition
// zero, producing single-caret underlines even when the range was known.
func RangeOf(n ast.Node) (Position, Position) {
	return PositionOf(n.Start()), EndPositionOf(n.End())
}

// String returns a human-readable representation of the severity.
func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	case Info:
		return "info"
	case Hint:
		return "hint"
	default:
		return "unknown"
	}
}

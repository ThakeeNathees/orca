// Package diagnostic defines the error/warning types used across the
// Orca compiler pipeline. Both the parser and analyzer produce Diagnostics,
// and the LSP server converts them to LSP protocol format for editors.
package diagnostic

import "fmt"

// Severity indicates how serious a diagnostic is.
type Severity int

const (
	Error   Severity = iota // a problem that prevents compilation
	Warning                 // a potential issue that doesn't block compilation
	Info                    // informational message
	Hint                    // a suggestion for improvement
)

// Position represents a source location in an .oc file.
// Line and Column are 1-based, matching the lexer's convention.
type Position struct {
	Line   int
	Column int
}

// Diagnostic represents a single compiler message (error, warning, etc.)
// tied to a source location. Used by the parser, analyzer, and codegen
// stages, then converted to LSP diagnostics for editor integration.
type Diagnostic struct {
	Severity Severity
	Position Position
	Message  string
	Source   string // which stage produced this: "parser", "analyzer", etc.
}

// Error implements the error interface so a Diagnostic can be used as a Go error.
func (d Diagnostic) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", d.Source, d.Position.Line, d.Position.Column, d.Message)
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

package diagnostic

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// tabWidth is how many columns a '\t' expands to when rendering a source
// snippet. Hard-coded rather than configurable — matches most editors'
// default and keeps the underline math simple.
const tabWidth = 4

// Render formats a Diagnostic as a multi-line snippet with a header line,
// a file:line:col location, and ±2 lines of source context with an
// underline marking the error range. EndPosition.Column is treated as
// exclusive (the position just past the last underlined character), which
// matches the lexer convention of range = [start, start+length).
//
// Colors are applied via fatih/color, which auto-disables when stdout is
// not a TTY (or when color.NoColor is set explicitly, e.g. in tests).
func Render(source string, d Diagnostic) string {
	header, tilde := severityColors(d.Severity)

	var b strings.Builder
	b.WriteString(header.Sprintf("%s[%s]", d.Severity.String(), d.Code))
	b.WriteString(header.Sprint(": "))
	b.WriteString(d.Message)
	b.WriteByte('\n')

	fmt.Fprintf(&b, "  --> %s:%d:%d\n", d.Position.File, d.Position.Line, d.Position.Column)

	// Split source into lines and drop a trailing empty element produced
	// by a final newline, so len(lines) matches 1-based line counts.
	lines := strings.Split(source, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return b.String()
	}

	errLine := d.Position.Line
	startLine := errLine - 2
	if startLine < 1 {
		startLine = 1
	}
	endLine := errLine + 2
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Gutter width = digits in the largest line number shown.
	width := len(strconv.Itoa(endLine))
	if width < 1 {
		width = 1
	}
	gutterEmpty := fmt.Sprintf(" %*s |", width, "")

	b.WriteString(gutterEmpty)
	b.WriteByte('\n')

	for ln := startLine; ln <= endLine; ln++ {
		srcLine := lines[ln-1]
		fmt.Fprintf(&b, " %*d | %s\n", width, ln, expandTabs(srcLine))

		if ln != errLine {
			continue
		}

		startCol := visualCol(srcLine, d.Position.Column)
		length := 0
		switch {
		case d.EndPosition.Line == d.Position.Line && d.EndPosition.Column > d.Position.Column:
			length = visualCol(srcLine, d.EndPosition.Column) - startCol
		case d.EndPosition.Line > d.Position.Line:
			// Multi-line range: underline from start to end of this line.
			length = visualCol(srcLine, len(srcLine)+1) - startCol
		}
		if length < 1 {
			length = 1
		}

		pad := strings.Repeat(" ", startCol-1)
		marks := tilde.Sprint(strings.Repeat("~", length))
		fmt.Fprintf(&b, "%s %s%s\n", gutterEmpty, pad, marks)
	}

	return b.String()
}

// severityColors returns the header and underline colors for a severity.
func severityColors(s Severity) (header, tilde *color.Color) {
	switch s {
	case Error:
		return color.New(color.FgRed, color.Bold), color.New(color.FgRed, color.Bold)
	case Warning:
		return color.New(color.FgYellow, color.Bold), color.New(color.FgYellow, color.Bold)
	case Info:
		return color.New(color.FgBlue, color.Bold), color.New(color.FgBlue, color.Bold)
	case Hint:
		return color.New(color.FgCyan, color.Bold), color.New(color.FgCyan, color.Bold)
	default:
		return color.New(color.Bold), color.New(color.Bold)
	}
}

// expandTabs replaces '\t' with spaces up to the next tab stop (tabWidth).
func expandTabs(line string) string {
	var b strings.Builder
	col := 0
	for _, r := range line {
		if r == '\t' {
			spaces := tabWidth - (col % tabWidth)
			b.WriteString(strings.Repeat(" ", spaces))
			col += spaces
			continue
		}
		b.WriteRune(r)
		col++
	}
	return b.String()
}

// visualCol maps a 1-based original column in line to a 1-based visual
// column after tab expansion. If col is past the end of the line, it
// returns the visual column just past the last character.
func visualCol(line string, col int) int {
	visual := 1
	i := 1
	for _, r := range line {
		if i >= col {
			return visual
		}
		if r == '\t' {
			visual += tabWidth - ((visual - 1) % tabWidth)
		} else {
			visual++
		}
		i++
	}
	return visual
}

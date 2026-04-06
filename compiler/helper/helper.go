// Package helper provides shared utility functions used across the compiler.
package helper

import (
	"strings"
	"unicode"

	"github.com/thakee/orca/compiler/ast"
)

// ToPascalCase converts a snake_case identifier to PascalCase.
// Each segment separated by underscores is capitalized and joined.
//
//	"article"         → "Article"
//	"research_report" → "ResearchReport"
//	""                → ""
func ToPascalCase(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		b.WriteString(string(runes))
	}
	return b.String()
}

// HasAnnotation reports whether annotations contains a decorator with the
// given name (the identifier after @, without the @ prefix).
func HasAnnotation(annotations []*ast.Annotation, name string) bool {
	for _, ann := range annotations {
		if ann != nil && ann.Name == name {
			return true
		}
	}
	return false
}

// Package helper provides standalone string utilities used across the
// compiler. By design, this package depends only on the Go standard library
// — no other compiler module — so it can be imported from anywhere without
// risk of cycles.
package helper

import (
	"strings"
	"unicode"
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

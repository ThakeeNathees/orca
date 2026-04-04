// Package helper provides small, generic utility functions shared across
// the Orca compiler pipeline. Functions here must be pure utilities with
// no domain-specific logic — if it knows about AST nodes, tokens, or
// diagnostics, it belongs in the relevant package instead.
package helper

import (
	"fmt"
	"sort"
	"strings"
)

// SortStrings sorts a slice of strings in place.
func SortStrings(s []string) {
	sort.Strings(s)
}

// JoinQuoted joins strings as comma-separated double-quoted values.
// e.g. ["A", "B"] → `"A", "B"`
func JoinQuoted(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = fmt.Sprintf("%q", n)
	}
	return strings.Join(quoted, ", ")
}

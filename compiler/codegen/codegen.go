// Package codegen defines shared utilities for code generation.
// Language-specific backends live in sub-packages (e.g., python/langgraph).
package codegen

import "fmt"

// SourceComment formats a source mapping comment like "agents.oc:42" or "line 42".
func SourceComment(file string, line int) string {
	if file != "" {
		return fmt.Sprintf("%s:%d", file, line)
	}
	return fmt.Sprintf("line %d", line)
}

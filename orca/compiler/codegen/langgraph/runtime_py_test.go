package langgraph

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// findPythonWithPydantic returns the first python executable that can import
// pydantic, preferring the locally built venv so contributors don't need a
// system-wide install. Returns "" if none is available.
func findPythonWithPydantic() string {
	candidates := []string{
		"bin/build/.venv/bin/python",
		"../../bin/build/.venv/bin/python",
		"python3",
		"python",
	}
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		cmd := exec.Command(abs, "-c", "import pydantic")
		if err := cmd.Run(); err == nil {
			return abs
		}
		// Fall back to PATH lookup for unqualified names.
		if !strings.Contains(c, "/") {
			if _, err := exec.LookPath(c); err == nil {
				cmd := exec.Command(c, "-c", "import pydantic")
				if err := cmd.Run(); err == nil {
					return c
				}
			}
		}
	}
	return ""
}

// TestRuntimeCoerceOutputSchema exercises _orca__coerce_output_schema by
// running runtime_test.py. Skips if no python with pydantic is available.
func TestRuntimeCoerceOutputSchema(t *testing.T) {
	python := findPythonWithPydantic()
	if python == "" {
		t.Skip("no python with pydantic available; skipping runtime coerce test")
	}

	script, err := filepath.Abs("runtime_test.py")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	cmd := exec.Command(python, script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("runtime_test.py failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "OK") {
		t.Fatalf("expected OK, got:\n%s", out)
	}
}

package codegen_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/codegen/langgraph"
	"github.com/thakee/orca/compiler/codegen/python"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

// buildFromSource parses, analyzes, and generates output directly from the AST.
func buildFromSource(t *testing.T, source string) python.Output {
	t.Helper()
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	result := analyzer.Analyze(program)
	if len(result.Diagnostics) > 0 {
		t.Fatalf("analyzer diagnostics: %v", result.Diagnostics)
	}
	backend := langgraph.New(program)
	return backend.Generate()
}

// TestGoldenFiles runs golden file tests for codegen. Each test case
// has a .oc input, a .py expected output, and a .toml expected output.
// Run with -update-golden to regenerate expected files.
func TestGoldenFiles(t *testing.T) {
	goldenDir := "testdata/golden"
	inputs, err := filepath.Glob(filepath.Join(goldenDir, "*.oc"))
	if err != nil {
		t.Fatalf("failed to glob golden inputs: %v", err)
	}
	if len(inputs) == 0 {
		t.Fatal("no golden .oc files found")
	}

	for _, inputPath := range inputs {
		name := strings.TrimSuffix(filepath.Base(inputPath), ".oc")
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatalf("failed to read %s: %v", inputPath, err)
			}

			output := buildFromSource(t, string(source))

			pyPath := filepath.Join(goldenDir, name+".py")
			tomlPath := filepath.Join(goldenDir, name+".toml")

			if *updateGolden {
				if err := os.WriteFile(pyPath, []byte(output.MainPy), 0644); err != nil {
					t.Fatalf("failed to update %s: %v", pyPath, err)
				}
				if err := os.WriteFile(tomlPath, []byte(output.PyProjectTOML), 0644); err != nil {
					t.Fatalf("failed to update %s: %v", tomlPath, err)
				}
				return
			}

			expectedPy, err := os.ReadFile(pyPath)
			if err != nil {
				t.Fatalf("failed to read %s (run with -update-golden to create): %v", pyPath, err)
			}
			if output.MainPy != string(expectedPy) {
				t.Errorf("main.py mismatch for %s:\n--- expected ---\n%s\n--- got ---\n%s",
					name, string(expectedPy), output.MainPy)
			}

			expectedTOML, err := os.ReadFile(tomlPath)
			if err != nil {
				t.Fatalf("failed to read %s (run with -update-golden to create): %v", tomlPath, err)
			}
			if output.PyProjectTOML != string(expectedTOML) {
				t.Errorf("pyproject.toml mismatch for %s:\n--- expected ---\n%s\n--- got ---\n%s",
					name, string(expectedTOML), output.PyProjectTOML)
			}
		})
	}
}

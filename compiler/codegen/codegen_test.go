package codegen_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/codegen"
	"github.com/thakee/orca/compiler/codegen/langgraph"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

// buildFromSource parses, analyzes, and generates output directly from the AST.
func buildFromSource(t *testing.T, source string) codegen.CodegenOutput {
	t.Helper()
	l := lexer.New(source, "")
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	result := analyzer.Analyze(program)
	if len(result.Diagnostics) > 0 {
		t.Fatalf("analyzer diagnostics: %v", result.Diagnostics)
	}
	backend := langgraph.New(&result)
	return backend.Generate()
}

// fileMap flattens the output tree into a name→content map for easy lookup.
func fileMap(dir codegen.OutputDirectory) map[string]string {
	m := make(map[string]string)
	for _, f := range dir.Files {
		m[f.Name] = f.Content
	}
	for _, d := range dir.Directories {
		sub := fileMap(d)
		for k, v := range sub {
			m[d.Name+"/"+k] = v
		}
	}
	return m
}

// goldenExtensions maps golden file extensions to output file names.
var goldenExtensions = map[string]string{
	".py":   "main.py",
	".toml": "pyproject.toml",
}

// TestGoldenFiles runs golden file tests for codegen. Each test case
// has a .orca input, a .py expected output, and a .toml expected output.
// Run with -update-golden to regenerate expected files.
func TestGoldenFiles(t *testing.T) {
	goldenDir := "testdata/golden"
	inputs, err := filepath.Glob(filepath.Join(goldenDir, "*.orca"))
	if err != nil {
		t.Fatalf("failed to glob golden inputs: %v", err)
	}
	if len(inputs) == 0 {
		t.Fatal("no golden .orca files found")
	}

	for _, inputPath := range inputs {
		name := strings.TrimSuffix(filepath.Base(inputPath), ".orca")
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatalf("failed to read %s: %v", inputPath, err)
			}

			output := buildFromSource(t, string(source))
			files := fileMap(output.RootDir)

			for ext, outputName := range goldenExtensions {
				goldenPath := filepath.Join(goldenDir, name+ext)
				got := files[outputName]

				if *updateGolden {
					if err := os.WriteFile(goldenPath, []byte(got), 0644); err != nil {
						t.Fatalf("failed to update %s: %v", goldenPath, err)
					}
					continue
				}

				expected, err := os.ReadFile(goldenPath)
				if err != nil {
					t.Fatalf("failed to read %s (run with -update-golden to create): %v", goldenPath, err)
				}
				if got != string(expected) {
					t.Errorf("%s mismatch for %s:\n--- expected ---\n%s\n--- got ---\n%s",
						outputName, name, string(expected), got)
				}
			}
		})
	}
}

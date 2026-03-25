package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/codegen/langgraph"
	"github.com/thakee/orca/compiler/diagnostic"
	"github.com/thakee/orca/compiler/ir"
	"github.com/thakee/orca/compiler/lexer"
	"github.com/thakee/orca/compiler/parser"
)

// buildCmd compiles all .oc files in the current directory.
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Compile .oc files to Python",
	Long:  "Reads all .oc files in the current directory, parses them, and produces a build/ folder with generated Python code.",
	RunE:  runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

// runBuild is the entry point for `orca build`.
func runBuild(cmd *cobra.Command, args []string) error {
	files, err := filepath.Glob("*.oc")
	if err != nil {
		return fmt.Errorf("failed to find .oc files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no .oc files found in current directory")
	}

	// Parse each file separately to preserve per-file line numbers.
	var program ast.Program
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		l := lexer.New(string(data))
		p := parser.New(l)
		fileProg := p.ParseProgram()

		if len(p.Diagnostics()) > 0 {
			for _, d := range p.Diagnostics() {
				fmt.Fprintf(os.Stderr, "%s:%s\n", file, d.Error())
			}
			return fmt.Errorf("compilation failed with parse errors")
		}

		// Tag each block with its source file for source mapping.
		for _, stmt := range fileProg.Statements {
			if block, ok := stmt.(*ast.BlockStatement); ok {
				block.SourceFile = file
			}
		}

		program.Statements = append(program.Statements, fileProg.Statements...)
	}

	// Run semantic analysis.
	result := analyzer.Analyze(&program)
	if len(result.Diagnostics) > 0 {
		hasError := false
		for _, d := range result.Diagnostics {
			fmt.Fprintln(os.Stderr, d.Error())
			if d.Severity == diagnostic.Error {
				hasError = true
			}
		}
		if hasError {
			return fmt.Errorf("compilation failed with analysis errors")
		}
	}

	// Build IR and generate code.
	built := ir.Build(&program)
	backend := langgraph.New(built)
	output := backend.Generate()

	// Write build/ directory.
	if err := os.MkdirAll("build", 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	if err := os.WriteFile("build/main.py", []byte(output.MainPy), 0644); err != nil {
		return fmt.Errorf("failed to write main.py: %w", err)
	}

	if err := os.WriteFile("build/pyproject.toml", []byte(output.PyProjectTOML), 0644); err != nil {
		return fmt.Errorf("failed to write pyproject.toml: %w", err)
	}

	fmt.Printf("compiled %d .oc file(s) → build/main.py, build/pyproject.toml\n", len(files))
	return nil
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thakee/orca/compiler/analyzer"
	"github.com/thakee/orca/compiler/ast"
	"github.com/thakee/orca/compiler/codegen"
	"github.com/thakee/orca/compiler/codegen/langgraph"
	"github.com/thakee/orca/compiler/diagnostic"
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

		l := lexer.New(string(data), file)
		p := parser.New(l)
		fileProg := p.ParseProgram()

		if len(p.Diagnostics()) > 0 {
			for _, d := range p.Diagnostics() {
				fmt.Fprintf(os.Stderr, "%s:%s\n", file, d.Error())
			}
			return fmt.Errorf("compilation failed with parse errors")
		}

		program.Statements = append(program.Statements, fileProg.Statements...)
	}

	// Run semantic analysis.
	analyzedProg := analyzer.Analyze(&program)
	if len(analyzedProg.Diagnostics) > 0 {
		hasError := false
		for _, d := range analyzedProg.Diagnostics {
			fmt.Fprintln(os.Stderr, d.Error())
			if d.Severity == diagnostic.Error {
				hasError = true
			}
		}
		if hasError {
			return fmt.Errorf("compilation failed with analysis errors")
		}
	}

	// Generate code directly from the analyzed AST.
	backend := langgraph.New(&analyzedProg)
	output := backend.Generate()

	// Check for codegen diagnostics.
	if len(output.Diagnostics) > 0 {
		hasError := false
		for _, d := range output.Diagnostics {
			fmt.Fprintln(os.Stderr, d.Error())
			if d.Severity == diagnostic.Error {
				hasError = true
			}
		}
		if hasError {
			return fmt.Errorf("compilation failed with codegen errors")
		}
	}

	// Write output tree to disk.
	if err := writeOutputDir(".", output.RootDir); err != nil {
		return err
	}

	fmt.Printf("compiled %d .oc file(s) → %s/\n", len(files), output.RootDir.Name)
	return nil
}

// writeOutputDir recursively writes an OutputDirectory tree to disk under parent.
func writeOutputDir(parent string, dir codegen.OutputDirectory) error {
	dirPath := filepath.Join(parent, dir.Name)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}
	for _, f := range dir.Files {
		fPath := filepath.Join(dirPath, f.Name)
		if err := os.WriteFile(fPath, []byte(f.Content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", fPath, err)
		}
	}
	for _, sub := range dir.Directories {
		if err := writeOutputDir(dirPath, sub); err != nil {
			return err
		}
	}
	return nil
}

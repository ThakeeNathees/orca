package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thakee/orca/orca/compiler/analyzer"
	"github.com/thakee/orca/orca/compiler/ast"
	"github.com/thakee/orca/orca/compiler/codegen"
	"github.com/thakee/orca/orca/compiler/codegen/langgraph"
	"github.com/thakee/orca/orca/compiler/diagnostic"
	"github.com/thakee/orca/orca/compiler/lexer"
	"github.com/thakee/orca/orca/compiler/parser"
)

// buildCmd compiles all .orca files in the current directory.
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Compile .orca files to Python",
	Long:  "Reads all .orca files in the current directory, parses them, and produces a build/ folder with generated Python code.",
	RunE:  runBuild,
}

type compileResult struct {
	FileCount int
	OutputDir string
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

// runBuild is the entry point for `orca build`.
func runBuild(cmd *cobra.Command, args []string) error {
	result, err := compileCurrentDir()
	if err != nil {
		return err
	}

	fmt.Printf("compiled %d .orca file(s) → %s/\n", result.FileCount, result.OutputDir)
	return nil
}

// compileCurrentDir compiles all .orca files in the current directory and writes
// generated output to disk.
func compileCurrentDir() (compileResult, error) {
	files, err := filepath.Glob("*.orca")
	if err != nil {
		return compileResult{}, fmt.Errorf("failed to find .orca files: %w", err)
	}

	if len(files) == 0 {
		return compileResult{}, fmt.Errorf("no .orca files found in current directory")
	}

	// Parse each file separately to preserve per-file line numbers.
	// sources keeps each file's text keyed by path so analyzer/codegen
	// diagnostics (which reference blocks by SourceFile) can still be
	// rendered with source context after the parse loop finishes.
	sources := make(map[string]string, len(files))
	var program ast.Program
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return compileResult{}, fmt.Errorf("failed to read %s: %w", file, err)
		}
		src := string(data)
		sources[file] = src

		l := lexer.New(src, file)
		p := parser.New(l)
		fileProg := p.ParseProgram()

		if len(p.Diagnostics()) > 0 {
			for _, d := range p.Diagnostics() {
				fmt.Fprintln(os.Stderr, diagnostic.Render(src, d))
			}
			return compileResult{}, fmt.Errorf("compilation failed with parse errors")
		}

		program.Statements = append(program.Statements, fileProg.Statements...)
	}

	// Run semantic analysis.
	analyzedProg := analyzer.Analyze(&program)
	if len(analyzedProg.Diagnostics) > 0 {
		if reportDiagnostics(analyzedProg.Diagnostics, sources) {
			return compileResult{}, fmt.Errorf("compilation failed with analysis errors")
		}
	}

	// Generate code directly from the analyzed AST.
	backend := langgraph.New(&analyzedProg)
	output := backend.Generate()

	// Check for codegen diagnostics.
	if len(output.Diagnostics) > 0 {
		if reportDiagnostics(output.Diagnostics, sources) {
			return compileResult{}, fmt.Errorf("compilation failed with codegen errors")
		}
	}

	// Write output tree to disk.
	if err := writeOutputDir(".", output.RootDir); err != nil {
		return compileResult{}, err
	}

	return compileResult{
		FileCount: len(files),
		OutputDir: output.RootDir.Name,
	}, nil
}

// reportDiagnostics prints each diagnostic to stderr using diagnostic.Render
// when the source file is available, falling back to the plain one-line
// format otherwise. Returns true if any Error-severity diagnostic was seen.
func reportDiagnostics(diags []diagnostic.Diagnostic, sources map[string]string) bool {
	hasError := false
	for _, d := range diags {
		if src, ok := sources[d.Position.File]; ok && src != "" {
			fmt.Fprintln(os.Stderr, diagnostic.Render(src, d))
		} else {
			fmt.Fprintln(os.Stderr, d.Error())
		}
		if d.Severity == diagnostic.Error {
			hasError = true
		}
	}
	return hasError
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

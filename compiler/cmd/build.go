package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
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

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		l := lexer.New(string(data))
		p := parser.New(l)
		program := p.ParseProgram()

		if len(p.Diagnostics()) > 0 {
			for _, d := range p.Diagnostics() {
				fmt.Fprintf(os.Stderr, "%s:%s\n", file, d.Error())
			}
			return fmt.Errorf("compilation failed")
		}

		fmt.Printf("%s: %d block(s) parsed\n", file, len(program.Statements))
	}

	// TODO: run analyzer, then codegen
	fmt.Println("build complete (codegen not yet implemented)")
	return nil
}

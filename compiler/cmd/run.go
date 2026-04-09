package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// runCmd builds and then runs the compiled output.
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Build and run .oc files",
	Long:  "Compiles .oc files and executes the generated Python code.",
	RunE:  runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

var compileForRun = compileCurrentDir

var executeCommandInDir = func(dir, name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Dir = dir
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

// runRun is the entry point for `orca run`.
func runRun(cmd *cobra.Command, args []string) error {
	result, err := compileForRun()
	if err != nil {
		return err
	}

	fmt.Printf("compiled %d .oc file(s) → %s/\n", result.FileCount, result.OutputDir)

	if err := executeCommandInDir(result.OutputDir, "uv", "sync"); err != nil {
		return fmt.Errorf("failed to run uv sync: %w", err)
	}

	uvRunArgs := append([]string{"run", "main.py"}, args...)
	if err := executeCommandInDir(result.OutputDir, "uv", uvRunArgs...); err != nil {
		return fmt.Errorf("failed to run uv run main.py: %w", err)
	}

	return nil
}

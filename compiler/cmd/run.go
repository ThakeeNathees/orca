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

// runRun is the entry point for `orca run`.
func runRun(cmd *cobra.Command, args []string) error {
	if err := runBuild(cmd, args); err != nil {
		return err
	}

	pythonExe := os.Getenv("ORCA_PYTHON")
	if pythonExe == "" {
		pythonExe = "python"
		if _, err := exec.LookPath(pythonExe); err != nil {
			pythonExe = "python3"
		}
	}
	if _, err := exec.LookPath(pythonExe); err != nil {
		return fmt.Errorf("python executable not found: %w", err)
	}

	runCmd := exec.Command(pythonExe, "main.py")
	runCmd.Dir = "build"
	runCmd.Stdin = os.Stdin
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("failed to run generated Python: %w", err)
	}
	return nil
}

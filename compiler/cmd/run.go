package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// buildOutputDir is the default directory for generated Python output.
const buildOutputDir = "build"

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

	pythonExe, err := resolvePythonExecutable()
	if err != nil {
		return err
	}

	pythonCmd := exec.Command(pythonExe, "main.py")
	pythonCmd.Dir = buildOutputDir
	pythonCmd.Stdin = os.Stdin
	pythonCmd.Stdout = os.Stdout
	pythonCmd.Stderr = os.Stderr
	if err := pythonCmd.Run(); err != nil {
		return fmt.Errorf("failed to run generated Python: %w", err)
	}
	return nil
}

// resolvePythonExecutable selects a Python interpreter, honoring ORCA_PYTHON and
// falling back to python then python3 on PATH.
func resolvePythonExecutable() (string, error) {
	if override := os.Getenv("ORCA_PYTHON"); override != "" {
		if filepath.IsAbs(override) {
			if _, err := os.Stat(override); err != nil {
				return "", fmt.Errorf("python executable %q not found: %w", override, err)
			}
			return override, nil
		}
		path, err := exec.LookPath(override)
		if err != nil {
			return "", fmt.Errorf("python executable %q not found: %w", override, err)
		}
		return path, nil
	}

	for _, candidate := range []string{"python", "python3"} {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("python executable not found in PATH")
}

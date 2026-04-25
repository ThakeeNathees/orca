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
	Short: "Build and run .orca files",
	Long:  "Compiles .orca files and executes the generated Python code.",
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

// lookupUV resolves the `uv` binary on PATH. Swappable in tests.
var lookupUV = func() (string, error) { return exec.LookPath("uv") }

// uvNotFoundMessage is shown when `uv` is missing from PATH. The first line
// is a TODO placeholder for the future auto-bootstrap (download into
// ~/.orcalang/bin/uv); the second line is the manual fallback users can act
// on today.
const uvNotFoundMessage = `uv not found on PATH
  TODO: auto-download uv binary to ~/.orcalang/bin/uv
  For now, please install uv manually: https://docs.astral.sh/uv/getting-started/installation/`

// runRun is the entry point for `orca run`.
func runRun(cmd *cobra.Command, args []string) error {
	result, err := compileForRun()
	if err != nil {
		return err
	}

	fmt.Printf("compiled %d .orca file(s) → %s/\n", result.FileCount, result.OutputDir)

	if _, err := lookupUV(); err != nil {
		return fmt.Errorf("%s", uvNotFoundMessage)
	}

	if err := executeCommandInDir(result.OutputDir, "uv", "sync"); err != nil {
		return fmt.Errorf("failed to run uv sync: %w", err)
	}

	uvRunArgs := append([]string{"run", "main.py"}, args...)
	if err := executeCommandInDir(result.OutputDir, "uv", uvRunArgs...); err != nil {
		return fmt.Errorf("failed to run uv run main.py: %w", err)
	}

	return nil
}

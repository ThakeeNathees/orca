package cmd

import (
	"fmt"

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
	// TODO: call build, then execute generated Python
	fmt.Println("orca run: not yet implemented")
	return nil
}

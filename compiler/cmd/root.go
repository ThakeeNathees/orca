// Package cmd defines the CLI commands for the Orca compiler.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd is the top-level command. Running `orca` with no subcommand
// prints usage information.
var rootCmd = &cobra.Command{
	Use:           "orca",
	Short:         "The Orca compiler",
	Long:          "Orca is a declarative language for defining AI agents.\nCompiles .oc files to Python/LangGraph code.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command. Called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

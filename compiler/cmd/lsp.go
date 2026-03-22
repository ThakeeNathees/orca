package cmd

import (
	"github.com/spf13/cobra"
	"github.com/thakee/orca/compiler/lsp"
)

// lspCmd starts the Orca LSP server on stdin/stdout.
var lspCmd = &cobra.Command{
	Use:   "lsp",
	Short: "Start the Orca language server",
	Long:  "Starts the Orca LSP server communicating over stdin/stdout. Used by editors for diagnostics, completion, and other language features.",
	RunE:  runLsp,
}

func init() {
	rootCmd.AddCommand(lspCmd)
}

// runLsp starts the LSP server on stdio.
func runLsp(cmd *cobra.Command, args []string) error {
	server := lsp.NewServer()
	return server.RunStdio()
}

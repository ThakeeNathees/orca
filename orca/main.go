package main

import "github.com/thakee/orca/orca/cmd"

func main() {
	// Dispatch to the Cobra root command (build, run, lsp, etc.).
	cmd.Execute()
}

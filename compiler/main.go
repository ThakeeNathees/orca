package main

import "github.com/thakee/orca/compiler/cmd"

func main() {
	// Dispatch to the Cobra root command (build, run, lsp, etc.).
	cmd.Execute()
}

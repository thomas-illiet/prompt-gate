package main

import (
	"fmt"
	"os"

	"promptgate/backend/cmd"
)

// main executes the Prompt Gate CLI and exits non-zero on failure.
func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

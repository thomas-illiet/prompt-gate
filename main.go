package main

import (
	"fmt"
	"os"

	"promptgate/backend/cmd"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

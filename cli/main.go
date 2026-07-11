package main

import (
	"fmt"
	"os"

	"glue-cli/internal/command"
)

func main() {
	if err := command.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

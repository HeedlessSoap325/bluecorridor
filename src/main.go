package main

import (
	"fmt"
	"os"

	"github.com/heedlesssoap325/bluecorridor/commands"
)

func main() {
	if err := commands.HandleCommand(); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m%s\033[0m\n", err)
		os.Exit(1)
	}
}

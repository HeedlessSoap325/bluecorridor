package main

import (
	"os"

	"github.com/heedlesssoap325/bluecorridor/commands"
	"github.com/heedlesssoap325/bluecorridor/internal/printing"
)

func main() {
	if err := commands.HandleCommand(); err != nil {
		printing.PrintWithColoredForeground(os.Stderr, printing.ERROR, "%s\n", err)
		os.Exit(1)
	}
}

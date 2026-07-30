package main

import (
	"os"

	"github.com/heedlesssoap325/bluecorridor/commands"
	"github.com/heedlesssoap325/bluecorridor/internal/console"
)

func main() {
	if err := commands.HandleCommand(); err != nil {
		console.PrintWithColoredForeground(os.Stderr, console.ERROR, "%s", err)
		os.Exit(1)
	}
}

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// Top-level usage/help message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [arguments]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "    help    Print this message")
		fmt.Fprintln(os.Stderr, "    list    List all resources that can be migrated")
		fmt.Fprintln(os.Stderr, "\nUse \"<command> help\" for command-specific help.")
	}

	if len(os.Args) < 2 {
		flag.Usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "-h", "--help", "help":
		flag.Usage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		flag.Usage()
		os.Exit(1)
	}
}

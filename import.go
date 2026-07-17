package main

import (
	"flag"
	"fmt"
	"os"
)

func importCMD(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	file := flag.String("file", "docker-export.json", "The file from which to import docker")
	help := fs.Bool("help", false, "Print this message")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s import [options]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		fs.PrintDefaults()
	}

	fs.Parse(args)

	if *help {
		fs.Usage()
	}

	fmt.Fprintf(os.Stdout, "%s\n", *file)
}

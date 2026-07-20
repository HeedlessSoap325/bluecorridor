package main

import (
	"flag"
	"fmt"
	"os"
	"encoding/json"
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

	raw, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error occured while reading File %s: %s\n", *file, err)
		os.Exit(1)
	}

	var state DockerState
	
	if json.Unmarshal(raw, &state) != nil {
		fmt.Fprintf(os.Stderr, "Error occured while parsing JSON: %s\n", err)
		os.Exit(1)
	}
}

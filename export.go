package main

import (
	"flag"
	"fmt"
	"os"
)

func exportCMD(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	help := fs.Bool("help", false, "Print this message")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s export [options]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		fs.PrintDefaults()
	}

	fs.Parse(args)

	if *help {
		fs.Usage()
	}


	fmt.Println("Exporting.........")
}
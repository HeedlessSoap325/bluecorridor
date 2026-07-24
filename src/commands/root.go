package commands

import (
	"flag"
	"fmt"
	"os"

	"github.com/heedlesssoap325/bluecorridor/internal/docker"
)

func HandleCommand() error {
	// Top-level usage/help message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [arguments]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "    help    Print this message")
		fmt.Fprintln(os.Stderr, "    list    List all resources that can be migrated")
		fmt.Fprintln(os.Stderr, "    export  Export all resources that can be migrated")
		fmt.Fprintln(os.Stderr, "    import  Import all resources")
		fmt.Fprintln(os.Stderr, "\nUse \"<command> --help\" for command-specific help.")
	}

	if len(os.Args) < 2 {
		flag.Usage()
		return fmt.Errorf("Insufficient number of argumnets provided")
	}

	if err := docker.InitializeDockerClient(); err != nil {
		return fmt.Errorf("Error while initializing docker API client: %s", err)
	}
	defer docker.DeinitializeDockerClient()

	var err error = nil
	switch os.Args[1] {
	case "-h", "--help", "help":
		flag.Usage()

	case "list":
		err = handleList(os.Args[2:])

	case "export":
		err = handleExport(os.Args[2:])

	case "import":
		err = handleImport(os.Args[2:])

	default:
		flag.Usage()
		err = fmt.Errorf("Unknown command: %s", os.Args[1])
	}

	return err
}

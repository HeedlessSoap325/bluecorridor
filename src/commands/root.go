package commands

import (
	"flag"
	"fmt"
	"os"
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

	switch os.Args[1] {
	case "-h", "--help", "help":
		flag.Usage()

	case "list":
		listCMD(os.Args[2:])

	case "export":
		exportCMD(os.Args[2:])

	case "import":
		importCMD(os.Args[2:])

	default:
		flag.Usage()
		return fmt.Errorf("Unknown command: %s\n\n", os.Args[1])
	}

	return nil
}

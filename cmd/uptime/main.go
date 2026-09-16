// Command uptime provides the DeepFurry Uptime CLI.
package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
)

var (
	version = "dev"
	commit  = "unknown"
)

const helpText = `DeepFurry Uptime

Usage:
  uptime [command]

Commands:
  help       Show this help
  version    Show version information

Options:
  -h, --help    Show this help
  --version     Show version information

No arguments shows this help.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 1 {
		fmt.Fprintln(stderr, "error: expected at most one command or option; use 'uptime --help'")
		return 2
	}

	command := "help"
	if len(args) == 1 {
		command = args[0]
	}

	var err error
	switch command {
	case "help", "-h", "--help":
		_, err = io.WriteString(stdout, helpText)
	case "version", "--version":
		_, err = fmt.Fprintf(stdout, "DeepFurry Uptime %s\nGo: %s\nCommit: %s\n", version, runtime.Version(), commit)
	default:
		fmt.Fprintf(stderr, "error: unknown command or option %q; use 'uptime --help'\n", command)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: write output: %v\n", err)
		return 1
	}
	return 0
}

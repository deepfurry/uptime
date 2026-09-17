// Command uptime provides the DeepFurry Uptime CLI.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/deepfurry/uptime/internal/config"
)

var (
	version = "dev"
	commit  = "unknown"
)

const helpText = `DeepFurry Uptime

Usage:
  uptime [command]

Commands:
  help          Show this help
  version       Show version information
  config check  Validate configuration

Options:
  -h, --help    Show this help
  --version     Show version information

No arguments shows this help.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "config" {
		if len(args) < 2 || args[1] != "check" {
			return diagnostic(stderr, "error: expected 'uptime config check'; use 'uptime --help'", 2)
		}
		return checkConfig(args[2:], stdout, stderr)
	}
	if len(args) > 1 {
		return diagnostic(stderr, "error: unexpected arguments; use 'uptime --help'", 2)
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
		return diagnostic(stderr, "error: unknown command or option; use 'uptime --help'", 2)
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: write output: %v\n", err)
		return 1
	}
	return 0
}

const configHelp = `Usage:
  uptime config check [--config PATH]

Validate an Uptime configuration without starting the service.

Options:
  --config PATH   Configuration file (default "./uptime.yaml")
  -h, --help      Show this help
`

func checkConfig(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("config check", flag.ContinueOnError)
	// Flag errors can quote supplied values. Emit our own safe usage diagnostic.
	flags.SetOutput(io.Discard)
	path := flags.String("config", "./uptime.yaml", "configuration file")
	var help bool
	flags.BoolVar(&help, "h", false, "show help")
	flags.BoolVar(&help, "help", false, "show help")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return diagnostic(stderr, "error: invalid arguments; use 'uptime config check --help'", 2)
	}
	output := configHelp
	if !help {
		if _, err := config.LoadFile(*path); err != nil {
			return diagnostic(stderr, "config check: "+err.Error(), 1)
		}
		output = "configuration is valid\n"
	}
	if _, err := io.WriteString(stdout, output); err != nil {
		return diagnostic(stderr, "config check: cannot write output", 1)
	}
	return 0
}

func diagnostic(stderr io.Writer, message string, code int) int {
	if _, err := fmt.Fprintln(stderr, message); err != nil {
		return 1
	}
	return code
}

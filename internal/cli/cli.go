package cli

import (
	"fmt"
	"io"
	"strings"
)

const appName = "passage"

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

func Run(args []string, stdout io.Writer, stderr io.Writer, build BuildInfo) int {
	if len(args) == 0 {
		printHelp(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printHelp(stdout)
		return 0
	case "version", "-v", "--version":
		printVersion(stdout, build)
		return 0
	default:
		fmt.Fprintf(stderr, "%s: unknown command %q\n", appName, args[0])
		fmt.Fprintf(stderr, "Run `%s help` for usage.\n", appName)
		return 1
	}
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`
passage is the command line client for passage.md.

Usage:
  passage <command>

Commands:
  help      Show this help.
  version   Show build version.

More commands are coming in the Phase 2 agent access work.
`)+"\n")
}

func printVersion(w io.Writer, build BuildInfo) {
	fmt.Fprintf(w, "passage %s\n", build.Version)
	if build.Commit != "" && build.Commit != "unknown" {
		fmt.Fprintf(w, "commit %s\n", build.Commit)
	}
	if build.Date != "" && build.Date != "unknown" {
		fmt.Fprintf(w, "built %s\n", build.Date)
	}
}

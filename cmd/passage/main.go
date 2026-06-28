package main

import (
	"os"

	"github.com/owainlewis/passage-cli/internal/cli"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}))
}

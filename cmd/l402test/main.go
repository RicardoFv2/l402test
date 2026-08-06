package main

import (
	"fmt"
	"os"

	"github.com/RicardoFv2/l402test/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	root := cli.NewRoot(cli.BuildInfo{Version: version, Commit: commit, Date: date})
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "l402test:", err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"os"

	"github.com/mchowning/slack-pipe/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	rootCmd := cli.NewRootCmd(version, commit)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

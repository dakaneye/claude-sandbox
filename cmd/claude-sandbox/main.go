package main

import (
	"os"

	"github.com/samueldacanay/claude-sandbox/internal/cli"
)

var version = "dev"

func main() {
	cmd := cli.NewRootCommand(version)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

package main

import (
	"os"

	"github.com/momentmaker/kaijutsu/cli/internal/cli"
)

func main() {
	err := cli.NewRootCmd().Execute()
	os.Exit(cli.ExitCode(err))
}

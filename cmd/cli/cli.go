package main

import (
	"os"

	"github.com/fdsakk/csda/pkg/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}

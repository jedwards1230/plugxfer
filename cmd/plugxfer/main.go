package main

import (
	"os"

	"github.com/jedwards1230/plugxfer/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}

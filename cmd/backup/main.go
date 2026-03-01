package main

import (
	"os"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}

package cli

import (
	"fmt"
	"io"
)

func printUsage(out io.Writer) {
	fmt.Fprint(out, `Usage:
  backup run <daily|weekly|monthly> [restic-args...]
  backup restore <target> [restic-args...]

Commands:
  run      Run a cadence-tagged restic backup
  restore  Restore latest snapshot to a target directory
`)
}

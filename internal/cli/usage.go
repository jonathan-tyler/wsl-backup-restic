package cli

import (
	"fmt"
	"io"
)

func printUsage(out io.Writer) {
	fmt.Fprint(out, `Usage:
  backup run <daily|weekly|monthly> [restic-args...]
  backup restore <target> [--dry-run] [restic-args...]

Commands:
  run      Run all configured profiles for a cadence
  restore  Restore latest snapshot to a target directory

Environment:
  BACKUP_CONFIG  Optional config file path override
`)
}

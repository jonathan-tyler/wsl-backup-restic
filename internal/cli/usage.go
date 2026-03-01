package cli

import (
	"fmt"
	"io"
)

func printUsage(out io.Writer) {
	fmt.Fprint(out, `Usage:
  backup run <daily|weekly|monthly> [restic-args...]
  backup setup
  backup restore <target> [--dry-run] [restic-args...]

Commands:
  run      Run all configured profiles for a cadence (fast preflight only)
  setup    Install or align restic versions for configured profiles
  restore  Restore latest snapshot to a target directory

Environment:
  BACKUP_CONFIG  Optional config file path override
`)
}

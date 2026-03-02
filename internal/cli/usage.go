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
  BACKUP_CONFIG                  Optional config file path override
  RESTIC_PASSWORD                Restic password value (manual or scheduled use)
  WSL_BACKUP_RESTIC_PASSWORD_FILE  Preferred password file path for wsl-backup
  RESTIC_PASSWORD_FILE           Restic-compatible password file path
  CREDENTIALS_DIRECTORY          systemd credentials directory (uses credential name restic_password)
`)
}

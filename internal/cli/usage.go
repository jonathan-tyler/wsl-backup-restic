package cli

import (
	"fmt"
	"io"
)

func printUsage(out io.Writer) {
	fmt.Fprint(out, `Usage:
  wsl-backup run <daily|weekly|monthly> [restic-args...]
  wsl-backup setup
  wsl-backup restore <target> [--dry-run] [restic-args...]

Commands:
  run      Run all configured profiles for a cadence (preflight validates that cadence's repos; extra args pass to restic backup, e.g. --dry-run)
  setup    Install or align restic versions for configured profiles
  restore  Restore latest snapshot to a target directory

Environment:
  BACKUP_CONFIG                  Optional config file path override
  RESTIC_PASSWORD                Restic password value (manual or scheduled use; manual prompts use hidden input)
  WSL_BACKUP_PASSWORD_FILE       Preferred password file path
  WSL_BACKUP_RESTIC_PASSWORD_FILE  Legacy alias for preferred password file path
  RESTIC_PASSWORD_FILE           Restic-compatible password file path
  CREDENTIALS_DIRECTORY          systemd credentials directory (uses credential name restic_password)
`)
}

package run

import (
	"context"
	"strings"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/restic"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/system"
)

func executeProfileBackup(ctx context.Context, profileName string, resticArgs []string, runner restic.Executor, exec system.Executor) error {
	if strings.EqualFold(profileName, "windows") {
		return executeWindowsProfileBackup(ctx, resticArgs, exec)
	}

	return executeWSLProfileBackup(ctx, resticArgs, runner)
}

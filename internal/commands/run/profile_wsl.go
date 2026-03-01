package run

import (
	"context"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/restic"
)

func executeWSLProfileBackup(ctx context.Context, resticArgs []string, runner restic.Executor) error {
	return runner.Run(ctx, resticArgs...)
}

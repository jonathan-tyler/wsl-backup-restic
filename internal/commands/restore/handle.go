package restore

import (
	"context"

	"github.com/example/wsl-backup/internal/apperr"
	"github.com/example/wsl-backup/internal/restic"
)

func Handle(ctx context.Context, args []string, runner restic.Executor) error {
	if len(args) == 0 {
		return apperr.UsageError{Message: "missing target: expected restore target path"}
	}

	target := args[0]
	resticArgs := []string{"restore", "latest", "--target", target}
	if len(args) > 1 && args[1] == "--dry-run" {
		resticArgs = append(resticArgs, "--dry-run")
		resticArgs = append(resticArgs, args[2:]...)
	} else {
		resticArgs = append(resticArgs, args[1:]...)
	}

	return runner.Run(ctx, resticArgs...)
}

package run

import (
	"context"
	"fmt"

	"github.com/example/wsl-backup/internal/apperr"
	"github.com/example/wsl-backup/internal/restic"
)

func Handle(ctx context.Context, args []string, runner restic.Executor) error {
	if len(args) == 0 {
		return apperr.UsageError{Message: "missing cadence: expected one of daily, weekly, monthly"}
	}

	cadence := args[0]
	if !isValidCadence(cadence) {
		return apperr.UsageError{Message: fmt.Sprintf("invalid cadence %q: expected one of daily, weekly, monthly", cadence)}
	}

	resticArgs := []string{"backup", "--tag", "cadence=" + cadence}
	resticArgs = append(resticArgs, args[1:]...)

	return runner.Run(ctx, resticArgs...)
}

func isValidCadence(value string) bool {
	switch value {
	case "daily", "weekly", "monthly":
		return true
	default:
		return false
	}
}

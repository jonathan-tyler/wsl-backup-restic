package setup

import (
	"context"
	"io"
	"os"

	"github.com/example/wsl-backup/internal/apperr"
	"github.com/example/wsl-backup/internal/config"
	"github.com/example/wsl-backup/internal/prompt"
	"github.com/example/wsl-backup/internal/restic"
	"github.com/example/wsl-backup/internal/resticversion"
	"github.com/example/wsl-backup/internal/system"
)

type ConfigLoader interface {
	Load() (config.File, error)
}

type Dependencies struct {
	Loader  ConfigLoader
	System  system.Executor
	Confirm prompt.ConfirmFunc
}

func Handle(ctx context.Context, args []string, _ restic.Executor) error {
	if len(args) != 0 {
		return apperr.UsageError{Message: "setup does not take positional arguments"}
	}

	deps := Dependencies{
		Loader:  config.NewLoader(),
		System:  system.NewOSExecutor(os.Stdout, os.Stderr),
		Confirm: prompt.NewYesNoConfirm(os.Stdin, os.Stdout),
	}

	return HandleWith(ctx, deps)
}

func HandleWith(ctx context.Context, deps Dependencies) error {
	if deps.Loader == nil {
		deps.Loader = config.NewLoader()
	}
	if deps.System == nil {
		deps.System = system.NewOSExecutor(io.Discard, io.Discard)
	}
	if deps.Confirm == nil {
		deps.Confirm = func(string) (bool, error) { return false, nil }
	}

	cfg, err := deps.Loader.Load()
	if err != nil {
		return err
	}

	return resticversion.SyncInteractive(ctx, cfg, deps.System, deps.Confirm)
}

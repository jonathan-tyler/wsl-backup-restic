package setup

import (
	"context"
	"fmt"
	"os"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/apperr"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/prompt"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/restic"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/resticversion"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/system"
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

	fmt.Fprintln(os.Stdout, "Running backup setup checks and installers...")

	deps := Dependencies{
		Loader:  config.NewLoader(),
		System:  system.NewOSExecutor(os.Stdout, os.Stderr),
		Confirm: prompt.NewYesNoConfirm(os.Stdin, os.Stdout),
	}

	report, err := handleWithReport(ctx, deps)
	printSetupReport(report)
	if err == nil {
		fmt.Fprintln(os.Stdout, "Backup setup completed successfully.")
	} else {
		fmt.Fprintf(os.Stdout, "Backup setup failed: %v\n", err)
	}
	return err
}

func HandleWith(ctx context.Context, deps Dependencies) error {
	_, err := handleWithReport(ctx, deps)
	return err
}

func handleWithReport(ctx context.Context, deps Dependencies) (resticversion.SetupReport, error) {
	if deps.Loader == nil {
		deps.Loader = config.NewLoader()
	}
	if deps.System == nil {
		deps.System = system.NewOSExecutor(os.Stdout, os.Stderr)
	}
	if deps.Confirm == nil {
		deps.Confirm = func(string) (bool, error) { return false, nil }
	}

	cfg, err := deps.Loader.Load()
	if err != nil {
		return resticversion.SetupReport{}, err
	}

	return resticversion.SyncInteractiveWithReport(ctx, cfg, deps.System, deps.Confirm)
}

func printSetupReport(report resticversion.SetupReport) {
	if len(report.Items) == 0 {
		fmt.Fprintln(os.Stdout, "setup report: no profile checks were executed")
		return
	}

	fmt.Fprintln(os.Stdout, "setup report:")
	for _, item := range report.Items {
		fmt.Fprintf(os.Stdout, "- %s: %s (%s)\n", item.Platform, item.Status, item.Message)
	}
}

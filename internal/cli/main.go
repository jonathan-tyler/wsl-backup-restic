package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/example/wsl-backup/internal/apperr"
	"github.com/example/wsl-backup/internal/commands/restore"
	"github.com/example/wsl-backup/internal/commands/run"
	"github.com/example/wsl-backup/internal/platform"
	"github.com/example/wsl-backup/internal/restic"
)

func Main(args []string, stdout io.Writer, stderr io.Writer) int {
	r := Router{
		Stdout: stdout,
		Stderr: stderr,
		Runner: restic.NewOSRunner(stdout, stderr),
		Guard:  platform.NewWSLGuard(os.Getenv),
	}

	return r.Route(context.Background(), args)
}

type Router struct {
	Stdout io.Writer
	Stderr io.Writer
	Runner restic.Executor
	Guard  interface{ Validate() error }
	Run    func(context.Context, []string, restic.Executor) error
	Restore func(context.Context, []string, restic.Executor) error
}

func (r Router) Route(ctx context.Context, args []string) int {
	if len(args) == 0 {
		printUsage(r.Stderr)
		return 2
	}

	switch args[0] {
	case "help":
		printUsage(r.Stdout)
		return 0
	case "run":
		runHandler := r.Run
		if runHandler == nil {
			runHandler = run.Handle
		}
		if r.Guard != nil {
			if err := r.Guard.Validate(); err != nil {
				return r.renderError(err)
			}
		}
		if err := runHandler(ctx, args[1:], r.Runner); err != nil {
			return r.renderError(err)
		}
		return 0
	case "restore":
		restoreHandler := r.Restore
		if restoreHandler == nil {
			restoreHandler = restore.Handle
		}
		if r.Guard != nil {
			if err := r.Guard.Validate(); err != nil {
				return r.renderError(err)
			}
		}
		if err := restoreHandler(ctx, args[1:], r.Runner); err != nil {
			return r.renderError(err)
		}
		return 0
	default:
		fmt.Fprintf(r.Stderr, "unknown command: %s\n\n", args[0])
		printUsage(r.Stderr)
		return 2
	}
}

func (r Router) renderError(err error) int {
	var usageErr apperr.UsageError
	if errors.As(err, &usageErr) {
		fmt.Fprintf(r.Stderr, "%s\n\n", usageErr.Error())
		printUsage(r.Stderr)
		return 2
	}

	fmt.Fprintf(r.Stderr, "error: %s\n", err)
	return 1
}

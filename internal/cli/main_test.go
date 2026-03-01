package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/apperr"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/restic"
)

type fakeRunner struct {
	called bool
	args   []string
	err    error
}

func (f *fakeRunner) Run(_ context.Context, args ...string) error {
	f.called = true
	f.args = append([]string{}, args...)
	return f.err
}

func TestRouteReturnsUsageOnNoArgs(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	r := Router{Stdout: &stdout, Stderr: &stderr, Runner: &fakeRunner{}}

	code := r.Route(context.Background(), nil)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("expected usage text on stderr")
	}
}

func TestRouteHelpWritesUsageToStdout(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	r := Router{Stdout: &stdout, Stderr: &stderr, Runner: &fakeRunner{}}

	code := r.Route(context.Background(), []string{"help"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("expected usage text on stdout")
	}
}

func TestRouteUnknownCommand(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	r := Router{Stdout: &stdout, Stderr: &stderr, Runner: &fakeRunner{}}

	code := r.Route(context.Background(), []string{"wat"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("expected unknown command message")
	}
}

func TestRouteRunDispatchesToRunner(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	runner := &fakeRunner{}
	r := Router{Stdout: &stdout, Stderr: &stderr, Runner: runner, Run: func(ctx context.Context, args []string, execRunner restic.Executor) error {
		return execRunner.Run(ctx, "backup", "--tag", "cadence=daily")
	}}

	code := r.Route(context.Background(), []string{"run", "daily", "/tmp/source"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !runner.called {
		t.Fatalf("expected runner to be called")
	}
	if len(runner.args) < 3 || runner.args[0] != "backup" {
		t.Fatalf("unexpected run args: %#v", runner.args)
	}
}

func TestRouteRestoreDispatchesToRunner(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	runner := &fakeRunner{}
	r := Router{Stdout: &stdout, Stderr: &stderr, Runner: runner, Restore: func(ctx context.Context, args []string, execRunner restic.Executor) error {
		return execRunner.Run(ctx, "restore", "latest", "--target", "/tmp/restore")
	}}

	code := r.Route(context.Background(), []string{"restore", "/tmp/restore"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !runner.called {
		t.Fatalf("expected runner to be called")
	}
	if len(runner.args) < 4 || runner.args[0] != "restore" {
		t.Fatalf("unexpected restore args: %#v", runner.args)
	}
}

func TestRouteSetupDispatchesHandler(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	called := false
	r := Router{
		Stdout: &stdout,
		Stderr: &stderr,
		Runner: &fakeRunner{},
		Setup: func(ctx context.Context, args []string, execRunner restic.Executor) error {
			called = true
			if len(args) != 0 {
				t.Fatalf("expected empty args, got %#v", args)
			}
			return nil
		},
	}

	code := r.Route(context.Background(), []string{"setup"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !called {
		t.Fatalf("expected setup handler to be called")
	}
}

func TestRouteUsageErrorsReturnExitCode2(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	runner := &fakeRunner{err: apperr.UsageError{Message: "bad usage"}}
	r := Router{Stdout: &stdout, Stderr: &stderr, Runner: runner, Run: func(ctx context.Context, args []string, execRunner restic.Executor) error {
		return execRunner.Run(ctx, "backup")
	}}

	code := r.Route(context.Background(), []string{"run", "daily"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestRouteRuntimeErrorsReturnExitCode1(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	runner := &fakeRunner{err: errors.New("boom")}
	r := Router{Stdout: &stdout, Stderr: &stderr, Runner: runner, Run: func(ctx context.Context, args []string, execRunner restic.Executor) error {
		return execRunner.Run(ctx, "backup")
	}}

	code := r.Route(context.Background(), []string{"run", "daily", "/tmp/data"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

type fakeGuard struct{ err error }

func (g fakeGuard) Validate() error { return g.err }

func TestRouteRejectsUnsupportedEnvironment(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	r := Router{
		Stdout: &stdout,
		Stderr: &stderr,
		Runner: &fakeRunner{},
		Guard:  fakeGuard{err: fmt.Errorf("not wsl")},
		Run: func(ctx context.Context, args []string, execRunner restic.Executor) error {
			return nil
		},
	}

	code := r.Route(context.Background(), []string{"run", "daily"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not wsl") {
		t.Fatalf("expected environment error output")
	}
}

package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/example/wsl-backup/internal/apperr"
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
	r := Router{Stdout: &stdout, Stderr: &stderr, Runner: runner}

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
	r := Router{Stdout: &stdout, Stderr: &stderr, Runner: runner}

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

func TestRouteUsageErrorsReturnExitCode2(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	runner := &fakeRunner{err: apperr.UsageError{Message: "bad usage"}}
	r := Router{Stdout: &stdout, Stderr: &stderr, Runner: runner}

	code := r.Route(context.Background(), []string{"run", "daily"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestRouteRuntimeErrorsReturnExitCode1(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	runner := &fakeRunner{err: errors.New("boom")}
	r := Router{Stdout: &stdout, Stderr: &stderr, Runner: runner}

	code := r.Route(context.Background(), []string{"run", "daily", "/tmp/data"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

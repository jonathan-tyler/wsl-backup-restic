package setup

import (
	"context"
	"fmt"
	"testing"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/apperr"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
)

type fakeLoader struct {
	cfg config.File
	err error
}

func (l fakeLoader) Load() (config.File, error) {
	if l.err != nil {
		return config.File{}, l.err
	}
	return l.cfg, nil
}

type fakeSystem struct {
	runCaptureOutput string
	runCaptureErr    error
	runErr           error
}

func (s fakeSystem) Run(_ context.Context, _ string, _ ...string) error {
	return s.runErr
}

func (s fakeSystem) RunCapture(_ context.Context, _ string, _ ...string) (string, error) {
	return s.runCaptureOutput, s.runCaptureErr
}

func TestHandleRejectsPositionalArgs(t *testing.T) {
	err := Handle(context.Background(), []string{"extra"}, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, ok := err.(apperr.UsageError); !ok {
		t.Fatalf("expected usage error, got %T", err)
	}
}

func TestHandleWithRunsSyncAndPasses(t *testing.T) {
	deps := Dependencies{
		Loader: fakeLoader{cfg: config.File{
			ResticVersion: "0.18.1",
			Profiles: map[string]config.Profile{
				"wsl": {Repository: "/repo/wsl"},
			},
		}},
		System: fakeSystem{runCaptureOutput: "restic 0.18.1 compiled with go"},
		Confirm: func(string) (bool, error) {
			return true, nil
		},
	}

	err := HandleWith(context.Background(), deps)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestHandleWithReturnsLoaderError(t *testing.T) {
	err := HandleWith(context.Background(), Dependencies{Loader: fakeLoader{err: fmt.Errorf("load fail")}})
	if err == nil {
		t.Fatalf("expected error")
	}
}

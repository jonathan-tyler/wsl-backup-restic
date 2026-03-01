package unit

import (
	"context"
	"strings"
	"testing"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/commands/restore"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/commands/setup"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
)

type fakeRunner struct {
	args []string
	err  error
}

func (f *fakeRunner) Run(_ context.Context, args ...string) error {
	f.args = append([]string{}, args...)
	return f.err
}

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
	runCaptureOut map[string]string
	runCaptureErr map[string]error
	runCalls      []string
}

func (s *fakeSystem) Run(_ context.Context, name string, args ...string) error {
	s.runCalls = append(s.runCalls, name+" "+strings.Join(args, " "))
	return nil
}

func (s *fakeSystem) RunCapture(_ context.Context, name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	if s.runCaptureErr != nil {
		if err, ok := s.runCaptureErr[key]; ok {
			return "", err
		}
	}
	if s.runCaptureOut != nil {
		if out, ok := s.runCaptureOut[key]; ok {
			return out, nil
		}
	}
	return "", nil
}

func TestRestoreSupportsDryRun(t *testing.T) {
	runner := &fakeRunner{}
	err := restore.Handle(context.Background(), []string{"/tmp/restore", "--dry-run", "--include", "*.txt"}, runner)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	joined := strings.Join(runner.args, " ")
	if !strings.Contains(joined, "--dry-run") {
		t.Fatalf("expected --dry-run in restore args: %v", runner.args)
	}
}

func TestSetupRunsWithMatchingVersion(t *testing.T) {
	deps := setup.Dependencies{
		Loader: fakeLoader{cfg: config.File{
			ResticVersion: "0.18.1",
			Profiles: map[string]config.Profile{
				"wsl": {Repository: "/repo/wsl"},
			},
		}},
		System: &fakeSystem{runCaptureOut: map[string]string{"restic version": "restic 0.18.1 compiled with go"}},
		Confirm: func(string) (bool, error) {
			return true, nil
		},
	}

	err := setup.HandleWith(context.Background(), deps)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

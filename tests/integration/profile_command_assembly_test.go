package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/commands/run"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
)

type fakeRunner struct {
	calls [][]string
}

func (f *fakeRunner) Run(_ context.Context, args ...string) error {
	f.calls = append(f.calls, append([]string{}, args...))
	return nil
}

type fakeLoader struct {
	cfg config.File
}

func (l fakeLoader) Load() (config.File, error) {
	return l.cfg, nil
}

type fakeSystem struct {
	runCalls   [][]string
	runCapture map[string]string
}

func (s *fakeSystem) Run(_ context.Context, name string, args ...string) error {
	s.runCalls = append(s.runCalls, append([]string{name}, args...))
	return nil
}

func (s *fakeSystem) RunCapture(_ context.Context, name string, args ...string) (string, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	if out, ok := s.runCapture[key]; ok {
		return out, nil
	}
	return "", nil
}

func writeRules(t *testing.T, dir string, profile string, cadence string) {
	t.Helper()
	includePath := filepath.Join(dir, fmt.Sprintf("%s.include.%s.txt", profile, cadence))
	if err := os.WriteFile(includePath, []byte("/tmp/src-"+profile+"\n"), 0o644); err != nil {
		t.Fatalf("write include: %v", err)
	}

	excludePath := filepath.Join(dir, fmt.Sprintf("%s.exclude.%s.txt", profile, cadence))
	if err := os.WriteFile(excludePath, []byte("/tmp/exclude-"+profile+"\n"), 0o644); err != nil {
		t.Fatalf("write exclude: %v", err)
	}
}

func TestRunAssemblesProfileCommandsForWSLAndWindows(t *testing.T) {
	rulesDir := t.TempDir()
	writeRules(t, rulesDir, "wsl", "daily")
	writeRules(t, rulesDir, "windows", "daily")

	runner := &fakeRunner{}
	exec := &fakeSystem{runCapture: map[string]string{}}

	exec.runCapture["restic version"] = "restic 0.18.1 compiled with go"
	exec.runCapture["powershell.exe -NoProfile -Command restic version"] = "restic 0.18.1 compiled with go"
	exec.runCapture["wslpath -w "+filepath.Join(rulesDir, "windows.include.daily.txt")] = `C:\rules\windows.include.daily.txt`
	exec.runCapture["wslpath -w "+filepath.Join(rulesDir, "windows.exclude.daily.txt")] = `C:\rules\windows.exclude.daily.txt`

	loader := fakeLoader{cfg: config.FileWithPathForTest(config.File{
		ResticVersion: "0.18.1",
		Profiles: map[string]config.Profile{
			"wsl":     {Repository: "/repo/wsl"},
			"windows": {Repository: `C:\repo`, UseFSSnapshot: true},
		},
	}, filepath.Join(rulesDir, "config.yaml"))}

	err := run.HandleWith(context.Background(), []string{"daily", "--one-file-system"}, runner, run.RunDependencies{
		Loader: loader,
		Stat:   os.Stat,
		System: exec,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 wsl runner call, got %d", len(runner.calls))
	}
	if len(exec.runCalls) != 1 {
		t.Fatalf("expected 1 windows system call, got %d", len(exec.runCalls))
	}

	windowsCommand := strings.Join(exec.runCalls[0], " ")
	if !strings.Contains(windowsCommand, "restic.exe") {
		t.Fatalf("expected restic.exe command, got %v", exec.runCalls[0])
	}
	if !strings.Contains(windowsCommand, `C:\rules\windows.include.daily.txt`) {
		t.Fatalf("expected converted windows include path, got %v", exec.runCalls[0])
	}
}

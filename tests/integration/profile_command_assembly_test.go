package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/commands/run"
	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/config"
)

type fakeRunner struct {
	mu              sync.Mutex
	calls           [][]string
	includeContents []string
	snapshotRepos   []string
}

func (f *fakeRunner) Run(_ context.Context, args ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string{}, args...))
	for index := 0; index < len(args)-1; index++ {
		if args[index] == "--repo" && len(args) > 0 && args[0] == "snapshots" {
			f.snapshotRepos = append(f.snapshotRepos, args[index+1])
		}
		if args[index] != "--files-from" {
			continue
		}
		content, err := os.ReadFile(args[index+1])
		if err != nil {
			return err
		}
		f.includeContents = append(f.includeContents, string(content))
	}
	return nil
}

type fakeLoader struct {
	cfg config.File
}

func (l fakeLoader) Load() (config.File, error) {
	return l.cfg, nil
}

type fakeSystem struct {
	mu         sync.Mutex
	runCalls   [][]string
	runWithEnv []map[string]string
	runCapture map[string]string
}

func (s *fakeSystem) Run(_ context.Context, name string, args ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runCalls = append(s.runCalls, append([]string{name}, args...))
	s.runWithEnv = append(s.runWithEnv, map[string]string{})
	return nil
}

func (s *fakeSystem) RunWithEnv(_ context.Context, env map[string]string, name string, args ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runCalls = append(s.runCalls, append([]string{name}, args...))
	envCopy := map[string]string{}
	for key, value := range env {
		envCopy[key] = value
	}
	s.runWithEnv = append(s.runWithEnv, envCopy)
	return nil
}

func (s *fakeSystem) RunCapture(_ context.Context, name string, args ...string) (string, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	s.mu.Lock()
	defer s.mu.Unlock()
	if name == "wslpath" && len(args) == 2 && args[0] == "-w" {
		if out, ok := s.runCapture[key]; ok {
			return out, nil
		}
		return `C:\\converted\\path`, nil
	}
	if out, ok := s.runCapture[key]; ok {
		return out, nil
	}
	return "", nil
}

func writeRules(t *testing.T, dir string, cadence string) {
	t.Helper()
	includePath := filepath.Join(dir, fmt.Sprintf("includes.%s.txt", cadence))
	if err := os.WriteFile(includePath, []byte("/mnt/c/tmp/src\n"), 0o644); err != nil {
		t.Fatalf("write include: %v", err)
	}
}

func writeSharedExcludeRules(t *testing.T, dir string) {
	t.Helper()

	excludePath := filepath.Join(dir, "excludes.txt")
	if err := os.WriteFile(excludePath, []byte("/tmp/exclude\n"), 0o644); err != nil {
		t.Fatalf("write exclude: %v", err)
	}
}

func writeRepositoryConfig(t *testing.T, dir string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("repo-config"), 0o644); err != nil {
		t.Fatalf("write repository config: %v", err)
	}
}

func TestRunAssemblesProfileCommandsForWSLAndWindows(t *testing.T) {
	t.Setenv("RESTIC_PASSWORD", "test-password")

	rulesDir := t.TempDir()
	writeRules(t, rulesDir, "daily")
	writeSharedExcludeRules(t, rulesDir)

	wslRepo := t.TempDir()
	windowsRepo := t.TempDir()
	writeRepositoryConfig(t, wslRepo)
	writeRepositoryConfig(t, windowsRepo)

	runner := &fakeRunner{}
	exec := &fakeSystem{runCapture: map[string]string{}}

	exec.runCapture["restic version"] = "restic 0.18.1 compiled with go"
	exec.runCapture["pwsh.exe -NoProfile -Command restic version"] = "restic 0.18.1 compiled with go"

	loader := fakeLoader{cfg: config.FileWithPathForTest(config.File{
		ResticVersion: "0.18.1",
		Profiles: map[string]config.Profile{
			"wsl":     {Repository: wslRepo},
			"windows": {Repository: windowsRepo, UseFSSnapshot: false},
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

	if len(runner.calls) != 3 {
		t.Fatalf("expected 3 runner calls (wsl backup + 2 snapshots), got %d", len(runner.calls))
	}
	if len(exec.runCalls) != 1 {
		t.Fatalf("expected 1 windows system call, got %d", len(exec.runCalls))
	}

	windowsCommand := strings.Join(exec.runCalls[0], " ")
	if !strings.Contains(windowsCommand, "restic.exe") {
		t.Fatalf("expected restic.exe command, got %v", exec.runCalls[0])
	}
	if !strings.Contains(windowsCommand, `C:\\converted\\path`) {
		t.Fatalf("expected converted windows include path, got %v", exec.runCalls[0])
	}
	if strings.Contains(strings.Join(runner.calls[0], " "), filepath.Join(rulesDir, "includes.daily.txt")) {
		t.Fatalf("expected wsl command to use a staged include file, got %v", runner.calls[0])
	}
	if len(runner.includeContents) != 1 {
		t.Fatalf("expected captured staged include content, got %d entries", len(runner.includeContents))
	}
	if runner.includeContents[0] != "\n" {
		t.Fatalf("expected staged wsl include content to filter mounted windows paths, got %q", runner.includeContents[0])
	}
	if len(runner.snapshotRepos) != 2 {
		t.Fatalf("expected 2 snapshots repo calls, got %d", len(runner.snapshotRepos))
	}
	if !containsString(runner.snapshotRepos, wslRepo) {
		t.Fatalf("expected wsl snapshots repo %q, got %v", wslRepo, runner.snapshotRepos)
	}
	if !containsString(runner.snapshotRepos, windowsRepo) {
		t.Fatalf("expected windows snapshots repo %q, got %v", windowsRepo, runner.snapshotRepos)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

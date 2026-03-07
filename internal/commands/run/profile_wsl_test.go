package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type inspectingRunner struct {
	calls          [][]string
	includeContent string
	excludeContent string
}

func (r *inspectingRunner) Run(_ context.Context, args ...string) error {
	r.calls = append(r.calls, append([]string{}, args...))
	for index := 0; index < len(args)-1; index++ {
		switch args[index] {
		case "--files-from":
			content, err := os.ReadFile(args[index+1])
			if err != nil {
				return err
			}
			r.includeContent = string(content)
		case "--exclude-file":
			content, err := os.ReadFile(args[index+1])
			if err != nil {
				return err
			}
			r.excludeContent = string(content)
		}
	}
	return nil
}

func TestTranslateRuleFileContentForWSLFiltersWindowsIncludePaths(t *testing.T) {
	content := []byte("# comment\n/mnt/c/Users/daily/Data\n/tmp/wsl-only\nC:\\Users\\daily\\Docs\nrelative.txt\n")

	got := string(translateRuleFileContentForWSL(content, "--files-from"))
	want := "# comment\n\n/tmp/wsl-only\n\nrelative.txt\n"
	if got != want {
		t.Fatalf("unexpected translated content: got %q want %q", got, want)
	}
}

func TestTranslateRuleFileContentForWSLKeepsExcludePatterns(t *testing.T) {
	content := []byte("# comment\n/mnt/c/Users/daily/Data\nC:\\Users\\daily\\Docs\n*.tmp\n")

	got := string(translateRuleFileContentForWSL(content, "--exclude-file"))
	want := "# comment\n/mnt/c/Users/daily/Data\nC:\\Users\\daily\\Docs\n*.tmp\n"
	if got != want {
		t.Fatalf("unexpected translated content: got %q want %q", got, want)
	}
}

func TestExecuteWSLProfileBackupStagesFilteredRuleFiles(t *testing.T) {
	runner := &inspectingRunner{}
	rulesDir := t.TempDir()
	includePath := filepath.Join(rulesDir, "includes.daily.txt")
	excludePath := filepath.Join(rulesDir, "excludes.txt")
	if err := os.WriteFile(includePath, []byte("/mnt/c/Users/daily/Data\n/tmp/wsl-only\n"), 0o644); err != nil {
		t.Fatalf("write include file: %v", err)
	}
	if err := os.WriteFile(excludePath, []byte("*.tmp\n/mnt/c/Users/daily/Data\n"), 0o644); err != nil {
		t.Fatalf("write exclude file: %v", err)
	}

	originalCreateTemp := osCreateTemp
	createTempIndex := 0
	osCreateTemp = func(_ string, pattern string) (*os.File, error) {
		createTempIndex++
		path := filepath.Join(os.TempDir(), fmt.Sprintf("wsl-backup-orchestrator-rule-%d.txt", createTempIndex))
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
	t.Cleanup(func() {
		osCreateTemp = originalCreateTemp
		_ = os.Remove(filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-1.txt"))
		_ = os.Remove(filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-2.txt"))
	})

	args := []string{"--repo", "/repo", "backup", "--files-from", includePath, "--exclude-file", excludePath}
	err := executeWSLProfileBackup(context.Background(), args, runner)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected one runner call, got %d", len(runner.calls))
	}
	if runner.calls[0][4] == includePath {
		t.Fatalf("expected staged include path, got original %q", runner.calls[0][4])
	}
	if runner.calls[0][6] != excludePath {
		t.Fatalf("expected original exclude path, got %q", runner.calls[0][6])
	}
	if runner.includeContent != "\n/tmp/wsl-only\n" {
		t.Fatalf("unexpected staged include content: %q", runner.includeContent)
	}
	if runner.excludeContent != "*.tmp\n/mnt/c/Users/daily/Data\n" {
		t.Fatalf("unexpected staged exclude content: %q", runner.excludeContent)
	}

	for _, path := range []string{runner.calls[0][4]} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("expected staged rule file cleanup, stat err=%v", statErr)
		}
	}
}

func TestExecuteWSLProfileBackupUsesRunner(t *testing.T) {
	runner := &fakeRunner{}
	args := []string{"backup", "--tag", "cadence=daily", "/data/src"}

	err := executeWSLProfileBackup(context.Background(), args, runner)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected one runner call, got %d", len(runner.calls))
	}
}

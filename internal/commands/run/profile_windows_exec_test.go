package run

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/system"
)

func TestExecuteWindowsProfileBackupRunsResticExe(t *testing.T) {
	originalLoad := loadWindowsProfilePassword
	loadWindowsProfilePassword = func(context.Context) (string, error) {
		return "test-password", nil
	}
	t.Cleanup(func() {
		loadWindowsProfilePassword = originalLoad
	})

	fakeExec := &fakeSystem{runCapture: map[string]string{}}
	rulesDir := t.TempDir()
	includePath := filepath.Join(rulesDir, "includes.daily.txt")
	if err := os.WriteFile(includePath, []byte("/mnt/c/Users/daily/Data\n"), 0o644); err != nil {
		t.Fatalf("write include file: %v", err)
	}

	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-000.txt")] = "C:\\Temp\\wsl-backup-orchestrator-rule-000.txt"
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-000.txt")] = "C:\\Temp\\wsl-backup-orchestrator-password-000.txt"
	args := []string{"--repo", `C:\repo`, "backup", "--files-from", includePath}

	originalCreateTemp := osCreateTemp
	osCreateTemp = func(_ string, pattern string) (*os.File, error) {
		path := filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-000.txt")
		if strings.Contains(pattern, "rule") {
			path = filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-000.txt")
		}
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
	t.Cleanup(func() {
		osCreateTemp = originalCreateTemp
	})

	err := executeWindowsProfileBackup(context.Background(), args, false, fakeExec)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(fakeExec.runCalls) != 1 {
		t.Fatalf("expected one run call, got %d", len(fakeExec.runCalls))
	}
	joined := strings.Join(fakeExec.runCalls[0], " ")
	if !strings.Contains(joined, "restic.exe") {
		t.Fatalf("expected restic.exe call, got %v", fakeExec.runCalls[0])
	}
	if !strings.Contains(joined, `--password-file C:\Temp\wsl-backup-orchestrator-password-000.txt`) {
		t.Fatalf("expected windows password file arg, got %v", fakeExec.runCalls[0])
	}
	if !strings.Contains(joined, `C:\Temp\wsl-backup-orchestrator-rule-000.txt`) {
		t.Fatalf("expected converted path, got %v", fakeExec.runCalls[0])
	}
	if len(fakeExec.runWithEnv) != 1 {
		t.Fatalf("expected one run call metadata entry, got %d", len(fakeExec.runWithEnv))
	}
	if len(fakeExec.runWithEnv[0]) != 0 {
		t.Fatalf("expected no env overrides, got %v", fakeExec.runWithEnv[0])
	}

	passwordPath := filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-000.txt")
	if _, statErr := os.Stat(passwordPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected temporary password file cleanup, stat err=%v", statErr)
	}
	rulePath := filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-000.txt")
	if _, statErr := os.Stat(rulePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected temporary rule file cleanup, stat err=%v", statErr)
	}
}

func TestExecuteWindowsProfileBackupConvertsWSLRepositoryPath(t *testing.T) {
	originalLoad := loadWindowsProfilePassword
	loadWindowsProfilePassword = func(context.Context) (string, error) {
		return "test-password", nil
	}
	t.Cleanup(func() {
		loadWindowsProfilePassword = originalLoad
	})

	fakeExec := &fakeSystem{runCapture: map[string]string{}}
	rulesDir := t.TempDir()
	includePath := filepath.Join(rulesDir, "includes.daily.txt")
	if err := os.WriteFile(includePath, []byte("/mnt/c/Users/daily/Data\n"), 0o644); err != nil {
		t.Fatalf("write include file: %v", err)
	}

	fakeExec.runCapture["wslpath -w /mnt/c/repo"] = "C:\\repo"
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-000.txt")] = "C:\\Temp\\wsl-backup-orchestrator-rule-000.txt"
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-000.txt")] = "C:\\Temp\\wsl-backup-orchestrator-password-000.txt"
	args := []string{"--repo", "/mnt/c/repo", "backup", "--files-from", includePath}

	originalCreateTemp := osCreateTemp
	osCreateTemp = func(_ string, pattern string) (*os.File, error) {
		path := filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-000.txt")
		if strings.Contains(pattern, "rule") {
			path = filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-000.txt")
		}
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
	t.Cleanup(func() {
		osCreateTemp = originalCreateTemp
	})

	err := executeWindowsProfileBackup(context.Background(), args, false, fakeExec)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(fakeExec.runCalls) != 1 {
		t.Fatalf("expected one run call, got %d", len(fakeExec.runCalls))
	}
	joined := strings.Join(fakeExec.runCalls[0], " ")
	if !strings.Contains(joined, `--repo C:\repo`) && !strings.Contains(joined, `--repo C:\\repo`) {
		t.Fatalf("expected converted windows repository path, got %v", fakeExec.runCalls[0])
	}
	if strings.Contains(joined, "--repo /mnt/c/repo") {
		t.Fatalf("expected WSL repository path to be converted, got %v", fakeExec.runCalls[0])
	}
	if len(fakeExec.runWithEnv) != 1 {
		t.Fatalf("expected one run call metadata entry, got %d", len(fakeExec.runWithEnv))
	}
	if len(fakeExec.runWithEnv[0]) != 0 {
		t.Fatalf("expected no env overrides, got %v", fakeExec.runWithEnv[0])
	}

	passwordPath := filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-000.txt")
	if _, statErr := os.Stat(passwordPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected temporary password file cleanup, stat err=%v", statErr)
	}
	rulePath := filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-000.txt")
	if _, statErr := os.Stat(rulePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected temporary rule file cleanup, stat err=%v", statErr)
	}
}

func TestTranslateRuleFileContentForWindowsConvertsAndFiltersIncludePaths(t *testing.T) {
	content := []byte("# comment\n/mnt/c/Users/daily/Data\n/tmp/wsl-only\nC:\\Users\\daily\\Docs\nrelative.txt\n")

	got := string(translateRuleFileContentForWindows(content, "--files-from"))
	want := "# comment\nC:\\Users\\daily\\Data\n\nC:\\Users\\daily\\Docs\nrelative.txt\n"
	if got != want {
		t.Fatalf("unexpected translated content: got %q want %q", got, want)
	}
}

func TestTranslateRuleFileContentForWindowsHandlesPathsWithSpaces(t *testing.T) {
	content := []byte("# comment\n/mnt/c/Users/daily/My Docs/report 2026.txt\n/home/daily/My Data/report 2026.txt\nrelative path/with spaces.txt\n")

	got := string(translateRuleFileContentForWindows(content, "--files-from"))
	want := "# comment\nC:\\Users\\daily\\My Docs\\report 2026.txt\n\nrelative path/with spaces.txt\n"
	if got != want {
		t.Fatalf("unexpected translated content: got %q want %q", got, want)
	}
}

func TestTranslateRuleFileContentForWindowsKeepsExcludePatterns(t *testing.T) {
	content := []byte("# comment\n/mnt/c/Users/daily/Data\n/tmp/wsl-only\n*.tmp\n")

	got := string(translateRuleFileContentForWindows(content, "--exclude-file"))
	want := "# comment\nC:\\Users\\daily\\Data\n/tmp/wsl-only\n*.tmp\n"
	if got != want {
		t.Fatalf("unexpected translated content: got %q want %q", got, want)
	}
}

func TestExecuteWindowsProfileBackupFailsWhenPasswordEmpty(t *testing.T) {
	originalLoad := loadWindowsProfilePassword
	loadWindowsProfilePassword = func(context.Context) (string, error) {
		return "", nil
	}
	t.Cleanup(func() {
		loadWindowsProfilePassword = originalLoad
	})

	t.Setenv("RESTIC_PASSWORD", "")

	fakeExec := &fakeSystem{runCapture: map[string]string{}}
	rulesDir := t.TempDir()
	includePath := filepath.Join(rulesDir, "includes.daily.txt")
	if err := os.WriteFile(includePath, []byte("/mnt/c/Users/daily/Data\n"), 0o644); err != nil {
		t.Fatalf("write include file: %v", err)
	}

	err := executeWindowsProfileBackup(context.Background(), []string{"--repo", `C:\repo`, "backup", "--files-from", includePath}, false, fakeExec)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "restic password is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteWindowsProfileBackupRunsElevatedViaPowerShell(t *testing.T) {
	originalLoad := loadWindowsProfilePassword
	loadWindowsProfilePassword = func(context.Context) (string, error) {
		return "test-password", nil
	}
	originalResolve := resolveWindowsResticExecutable
	resolveWindowsResticExecutable = func(context.Context, system.Executor) (string, error) {
		return `C:\\Users\\daily\\scoop\\shims\\restic.exe`, nil
	}
	originalElevatedDir := elevatedWindowsTempDir
	elevatedWindowsTempDir = t.TempDir()
	originalElevatedPathToWindows := elevatedPathToWindows
	elevatedPathToWindows = func(path string) (string, bool) {
		return `C:\\Temp\\` + filepath.Base(path), true
	}
	t.Cleanup(func() {
		loadWindowsProfilePassword = originalLoad
		resolveWindowsResticExecutable = originalResolve
		elevatedWindowsTempDir = originalElevatedDir
		elevatedPathToWindows = originalElevatedPathToWindows
	})

	fakeExec := &fakeSystem{runCapture: map[string]string{}}
	rulesDir := t.TempDir()
	includePath := filepath.Join(rulesDir, "includes.daily.txt")
	if err := os.WriteFile(includePath, []byte("/mnt/c/Users/daily/Data\n"), 0o644); err != nil {
		t.Fatalf("write include file: %v", err)
	}

	args := []string{"--repo", `C:\repo`, "backup", "--files-from", includePath}

	originalCreateTemp := osCreateTemp
	osCreateTemp = func(_ string, pattern string) (*os.File, error) {
		path := filepath.Join(elevatedWindowsTempDir, "wsl-backup-orchestrator-password-111.txt")
		if strings.Contains(pattern, "rule") {
			path = filepath.Join(elevatedWindowsTempDir, "wsl-backup-orchestrator-rule-111.txt")
		}
		if strings.Contains(pattern, "exitcode") {
			path = filepath.Join(elevatedWindowsTempDir, "wsl-backup-orchestrator-exitcode-111.txt")
		}
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return nil, err
		}
		if strings.Contains(pattern, "exitcode") {
			if _, err := file.WriteString("0\n"); err != nil {
				_ = file.Close()
				return nil, err
			}
			if _, err := file.Seek(0, 0); err != nil {
				_ = file.Close()
				return nil, err
			}
		}
		return file, nil
	}
	t.Cleanup(func() {
		osCreateTemp = originalCreateTemp
	})

	err := executeWindowsProfileBackup(context.Background(), args, true, fakeExec)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(fakeExec.runCalls) != 1 {
		t.Fatalf("expected one run call, got %d", len(fakeExec.runCalls))
	}
	if fakeExec.runCalls[0][0] != "pwsh.exe" {
		t.Fatalf("expected pwsh.exe call, got %v", fakeExec.runCalls[0])
	}
	joined := strings.Join(fakeExec.runCalls[0], " ")
	if !strings.Contains(joined, "Start-Process") {
		t.Fatalf("expected elevated Start-Process command, got %v", fakeExec.runCalls[0])
	}
	if !strings.Contains(joined, "--password-file") {
		t.Fatalf("expected password-file argument in elevated command, got %v", fakeExec.runCalls[0])
	}
	if !strings.Contains(joined, `C:\\Users\\daily\\scoop\\shims\\restic.exe`) && !strings.Contains(joined, `C:\Users\daily\scoop\shims\restic.exe`) {
		t.Fatalf("expected resolved restic.exe full path in elevated command, got %v", fakeExec.runCalls[0])
	}

	passwordPath := filepath.Join(elevatedWindowsTempDir, "wsl-backup-orchestrator-password-111.txt")
	if _, statErr := os.Stat(passwordPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected elevated temporary password file cleanup, stat err=%v", statErr)
	}
	rulePath := filepath.Join(elevatedWindowsTempDir, "wsl-backup-orchestrator-rule-111.txt")
	if _, statErr := os.Stat(rulePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected elevated temporary rule file cleanup, stat err=%v", statErr)
	}
}

func TestRunWindowsResticCommandConvertsWSLRepositoryPathForInit(t *testing.T) {
	originalLoad := loadWindowsProfilePassword
	loadWindowsProfilePassword = func(context.Context) (string, error) {
		return "test-password", nil
	}
	t.Cleanup(func() {
		loadWindowsProfilePassword = originalLoad
	})

	fakeExec := &fakeSystem{runCapture: map[string]string{}}
	fakeExec.runCapture["wslpath -w /mnt/c/repo"] = "C:\\repo"
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-000.txt")] = "C:\\Temp\\wsl-backup-orchestrator-password-000.txt"

	originalCreateTemp := osCreateTemp
	osCreateTemp = func(_ string, pattern string) (*os.File, error) {
		path := filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-000.txt")
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
	t.Cleanup(func() {
		osCreateTemp = originalCreateTemp
	})

	err := runWindowsResticCommand(context.Background(), []string{"init", "--repo", "/mnt/c/repo"}, false, fakeExec)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(fakeExec.runCalls) != 1 {
		t.Fatalf("expected one run call, got %d", len(fakeExec.runCalls))
	}
	joined := strings.Join(fakeExec.runCalls[0], " ")
	if !strings.Contains(joined, `init --repo C:\repo`) && !strings.Contains(joined, `init --repo C:\\repo`) {
		t.Fatalf("expected converted windows repository path, got %v", fakeExec.runCalls[0])
	}
	if strings.Contains(joined, "init --repo /mnt/c/repo") {
		t.Fatalf("expected WSL repository path to be converted, got %v", fakeExec.runCalls[0])
	}
}

func TestRunWindowsResticCommandConvertsWSLRepositoryPathWithSpacesForInit(t *testing.T) {
	originalLoad := loadWindowsProfilePassword
	loadWindowsProfilePassword = func(context.Context) (string, error) {
		return "test-password", nil
	}
	t.Cleanup(func() {
		loadWindowsProfilePassword = originalLoad
	})

	fakeExec := &fakeSystem{runCapture: map[string]string{}}
	wslRepo := "/mnt/c/Backup Repo/Project Files"
	fakeExec.runCapture["wslpath -w "+wslRepo] = `C:\Backup Repo\Project Files`
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-000.txt")] = "C:\\Temp\\wsl-backup-orchestrator-password-000.txt"

	originalCreateTemp := osCreateTemp
	osCreateTemp = func(_ string, pattern string) (*os.File, error) {
		path := filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-000.txt")
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
	t.Cleanup(func() {
		osCreateTemp = originalCreateTemp
	})

	err := runWindowsResticCommand(context.Background(), []string{"init", "--repo", wslRepo}, false, fakeExec)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(fakeExec.runCalls) != 1 {
		t.Fatalf("expected one run call, got %d", len(fakeExec.runCalls))
	}
	joined := strings.Join(fakeExec.runCalls[0], " ")
	if !strings.Contains(joined, `init --repo C:\Backup Repo\Project Files`) && !strings.Contains(joined, `init --repo C:\\Backup Repo\\Project Files`) {
		t.Fatalf("expected converted windows repository path with spaces, got %v", fakeExec.runCalls[0])
	}
	if strings.Contains(joined, "init --repo "+wslRepo) {
		t.Fatalf("expected WSL repository path with spaces to be converted, got %v", fakeExec.runCalls[0])
	}
}

package run

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	fakeExec.runCapture["wslpath -w "+filepath.Join(rulesDir, "windows.include.daily.txt")] = "C:\\rules\\windows.include.daily.txt"
	fakeExec.runCapture["wslpath -w "+filepath.Join(rulesDir, "windows.exclude.daily.txt")] = "C:\\rules\\windows.exclude.daily.txt"
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-restic-password-000.txt")] = "C:\\Temp\\wsl-backup-restic-password-000.txt"
	args := []string{"--repo", `C:\repo`, "backup", "--files-from", filepath.Join(rulesDir, "windows.include.daily.txt")}

	originalCreateTemp := osCreateTemp
	osCreateTemp = func(_ string, _ string) (*os.File, error) {
		path := filepath.Join(os.TempDir(), "wsl-backup-restic-password-000.txt")
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
	t.Cleanup(func() {
		osCreateTemp = originalCreateTemp
		_ = os.Remove(filepath.Join(os.TempDir(), "wsl-backup-restic-password-000.txt"))
	})

	err := executeWindowsProfileBackup(context.Background(), args, fakeExec)
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
	if !strings.Contains(joined, `--password-file C:\Temp\wsl-backup-restic-password-000.txt`) {
		t.Fatalf("expected windows password file arg, got %v", fakeExec.runCalls[0])
	}
	if !strings.Contains(joined, `C:\rules\windows.include.daily.txt`) {
		t.Fatalf("expected converted path, got %v", fakeExec.runCalls[0])
	}
	if len(fakeExec.runWithEnv) != 1 {
		t.Fatalf("expected one run call metadata entry, got %d", len(fakeExec.runWithEnv))
	}
	if len(fakeExec.runWithEnv[0]) != 0 {
		t.Fatalf("expected no env overrides, got %v", fakeExec.runWithEnv[0])
	}
}

func TestConvertRuleFileArgsToWindowsPassesThroughWindowsStylePaths(t *testing.T) {
	fakeExec := &fakeSystem{runCapture: map[string]string{}}
	args := []string{"backup", "--files-from", `C:\rules\windows.include.daily.txt`}

	converted, err := convertRuleFileArgsToWindows(context.Background(), args, fakeExec)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if converted[2] != `C:\rules\windows.include.daily.txt` {
		t.Fatalf("unexpected conversion result: %v", converted)
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
	fakeExec.runCapture["wslpath -w "+filepath.Join(rulesDir, "windows.include.daily.txt")] = "C:\\rules\\windows.include.daily.txt"

	err := executeWindowsProfileBackup(context.Background(), []string{"--repo", `C:\repo`, "backup", "--files-from", filepath.Join(rulesDir, "windows.include.daily.txt")}, fakeExec)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "restic password is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

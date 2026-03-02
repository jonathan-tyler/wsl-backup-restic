package run

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/system"
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

	err := executeWindowsProfileBackup(context.Background(), []string{"--repo", `C:\repo`, "backup", "--files-from", filepath.Join(rulesDir, "windows.include.daily.txt")}, false, fakeExec)
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
	includePath := filepath.Join(rulesDir, "windows.include.daily.txt")
	if err := os.WriteFile(includePath, []byte("C:\\Users\\daily\\Data\n"), 0o644); err != nil {
		t.Fatalf("write include file: %v", err)
	}

	args := []string{"--repo", `C:\repo`, "backup", "--files-from", includePath}

	originalCreateTemp := osCreateTemp
	osCreateTemp = func(_ string, pattern string) (*os.File, error) {
		path := filepath.Join(elevatedWindowsTempDir, "wsl-backup-restic-password-111.txt")
		if strings.Contains(pattern, "rule") {
			path = filepath.Join(elevatedWindowsTempDir, "wsl-backup-rule-111.txt")
		}
		if strings.Contains(pattern, "exitcode") {
			path = filepath.Join(elevatedWindowsTempDir, "wsl-backup-restic-exitcode-111.txt")
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
		_ = os.Remove(filepath.Join(elevatedWindowsTempDir, "wsl-backup-restic-password-111.txt"))
		_ = os.Remove(filepath.Join(elevatedWindowsTempDir, "wsl-backup-rule-111.txt"))
		_ = os.Remove(filepath.Join(elevatedWindowsTempDir, "wsl-backup-restic-exitcode-111.txt"))
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
}

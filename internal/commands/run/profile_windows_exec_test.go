package run

import (
	"context"
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
	args := []string{"--repo", `C:\repo`, "backup", "--files-from", filepath.Join(rulesDir, "windows.include.daily.txt")}

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
	if !strings.Contains(joined, `C:\rules\windows.include.daily.txt`) {
		t.Fatalf("expected converted path, got %v", fakeExec.runCalls[0])
	}
	if len(fakeExec.runWithEnv) != 1 {
		t.Fatalf("expected one env call, got %d", len(fakeExec.runWithEnv))
	}
	if fakeExec.runWithEnv[0]["RESTIC_PASSWORD"] != "test-password" {
		t.Fatalf("expected restic password env to be set")
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

package resticversion

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
)

func TestCheckCompatibleFailsWithSetupHintOnWindowsMismatch(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureOutput: map[string]string{
			"pwsh.exe -NoProfile -Command restic version": "restic 0.17.3 compiled with go1.24",
		},
	}

	err := CheckCompatible(context.Background(), config.File{
		ResticVersion: "0.18.1",
		Profiles:      map[string]config.Profile{"windows": {Repository: `C:\repo`}},
	}, exec)

	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "run backup setup") {
		t.Fatalf("expected setup hint, got %v", err)
	}
}

func TestSyncInteractiveWindowsUpdateDeclinedReturnsError(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureOutput: map[string]string{
			"pwsh.exe -NoProfile -Command restic version": "restic 0.17.3 compiled with go1.24",
		},
		captureErr: map[string]error{},
		runErr:     map[string]error{},
	}

	err := SyncInteractive(context.Background(), config.File{
		ResticVersion: "0.18.1",
		Profiles:      map[string]config.Profile{"windows": {Repository: `C:\repo`}},
	}, exec, func(string) (bool, error) {
		return false, nil
	})

	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "version mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exec.runCalls) != 0 {
		t.Fatalf("did not expect update command when declined")
	}
}

func TestSyncInteractiveWithReportReturnsWindowsFailedStatusOnDecline(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureOutput: map[string]string{
			"pwsh.exe -NoProfile -Command restic version": "restic 0.17.3 compiled with go1.24",
		},
	}

	report, err := SyncInteractiveWithReport(context.Background(), config.File{
		ResticVersion: "0.18.1",
		Profiles:      map[string]config.Profile{"windows": {Repository: `C:\repo`}},
	}, exec, func(string) (bool, error) { return false, nil })

	if err == nil {
		t.Fatalf("expected error")
	}
	if len(report.Items) != 1 {
		t.Fatalf("expected 1 report item, got %d", len(report.Items))
	}
	if report.Items[0].Status != SetupFailed {
		t.Fatalf("expected failed status, got %s", report.Items[0].Status)
	}
}

func TestSyncWindowsInteractiveMissingConfirmError(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureErr: map[string]error{
			"pwsh.exe -NoProfile -Command restic version": errors.New("not found"),
		},
	}

	_, err := syncWindowsInteractive(context.Background(), "0.18.1", exec, func(string) (bool, error) {
		return false, errors.New("prompt failed")
	})

	if err == nil || !strings.Contains(err.Error(), "prompt failed") {
		t.Fatalf("expected prompt failure, got %v", err)
	}
}

func TestSyncWindowsInteractiveMissingDeclined(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureErr: map[string]error{
			"pwsh.exe -NoProfile -Command restic version": errors.New("not found"),
		},
	}

	report, err := syncWindowsInteractive(context.Background(), "0.18.1", exec, func(string) (bool, error) {
		return false, nil
	})

	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("expected required error, got %v", err)
	}
	if report.Status != SetupFailed {
		t.Fatalf("expected failed status, got %s", report.Status)
	}
}

func TestSyncWindowsInteractiveInstallFailure(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureErr: map[string]error{
			"pwsh.exe -NoProfile -Command restic version": errors.New("not found"),
		},
		runErr: map[string]error{
			"pwsh.exe -NoProfile -Command scoop install restic": errors.New("install fail"),
		},
	}

	report, err := syncWindowsInteractive(context.Background(), "0.18.1", exec, func(string) (bool, error) {
		return true, nil
	})

	if err == nil || !strings.Contains(err.Error(), "install fail") {
		t.Fatalf("expected install failure, got %v", err)
	}
	if report.Status != SetupFailed {
		t.Fatalf("expected failed status, got %s", report.Status)
	}
}

func TestSyncWindowsInteractiveMissingInstallSuccess(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureErr: map[string]error{
			"pwsh.exe -NoProfile -Command restic version": errors.New("not found"),
		},
	}

	report, err := syncWindowsInteractive(context.Background(), "0.18.1", exec, func(string) (bool, error) {
		return true, nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if report.Status != SetupInstalled {
		t.Fatalf("expected installed status, got %s", report.Status)
	}
}

func TestSyncWindowsInteractiveParseFailure(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureOutput: map[string]string{
			"pwsh.exe -NoProfile -Command restic version": "no version",
		},
	}

	report, err := syncWindowsInteractive(context.Background(), "0.18.1", exec, func(string) (bool, error) {
		return true, nil
	})

	if err == nil || !strings.Contains(err.Error(), "parse windows restic version") {
		t.Fatalf("expected parse error, got %v", err)
	}
	if report.Status != SetupFailed {
		t.Fatalf("expected failed status, got %s", report.Status)
	}
}

func TestSyncWindowsInteractiveMismatchConfirmError(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureOutput: map[string]string{
			"pwsh.exe -NoProfile -Command restic version": "restic 0.17.3 compiled with go1.24",
		},
	}

	_, err := syncWindowsInteractive(context.Background(), "0.18.1", exec, func(string) (bool, error) {
		return false, errors.New("prompt failed")
	})

	if err == nil || !strings.Contains(err.Error(), "prompt failed") {
		t.Fatalf("expected prompt failure, got %v", err)
	}
}

func TestSyncWindowsInteractiveMismatchUpdateFailure(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureOutput: map[string]string{
			"pwsh.exe -NoProfile -Command restic version": "restic 0.17.3 compiled with go1.24",
		},
		runErr: map[string]error{
			"pwsh.exe -NoProfile -Command scoop update restic": errors.New("update fail"),
		},
	}

	report, err := syncWindowsInteractive(context.Background(), "0.18.1", exec, func(string) (bool, error) {
		return true, nil
	})

	if err == nil || !strings.Contains(err.Error(), "update fail") {
		t.Fatalf("expected update failure, got %v", err)
	}
	if report.Status != SetupFailed {
		t.Fatalf("expected failed status, got %s", report.Status)
	}
}

func TestSyncWindowsInteractiveMismatchUpdateSuccess(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureOutput: map[string]string{
			"pwsh.exe -NoProfile -Command restic version": "restic 0.17.3 compiled with go1.24",
		},
	}

	report, err := syncWindowsInteractive(context.Background(), "0.18.1", exec, func(string) (bool, error) {
		return true, nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if report.Status != SetupUpdated {
		t.Fatalf("expected updated status, got %s", report.Status)
	}
}

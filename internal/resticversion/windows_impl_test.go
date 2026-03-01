package resticversion

import (
	"context"
	"strings"
	"testing"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
)

func TestCheckCompatibleFailsWithSetupHintOnWindowsMismatch(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureOutput: map[string]string{
			"powershell.exe -NoProfile -Command restic version": "restic 0.17.3 compiled with go1.24",
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
			"powershell.exe -NoProfile -Command restic version": "restic 0.17.3 compiled with go1.24",
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
			"powershell.exe -NoProfile -Command restic version": "restic 0.17.3 compiled with go1.24",
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

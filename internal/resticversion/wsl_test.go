package resticversion

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
)

func TestCheckCompatibleFailsWithSetupHintOnWSLMismatch(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureOutput: map[string]string{
			"restic version": "restic 0.17.3 compiled with go1.24",
		},
	}

	err := CheckCompatible(context.Background(), config.File{
		ResticVersion: "0.18.1",
		Profiles:      map[string]config.Profile{"wsl": {Repository: "/repo/wsl"}},
	}, exec)

	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "run backup setup") {
		t.Fatalf("expected setup hint, got %v", err)
	}
}

func TestSyncInteractiveInstallsWSLWhenMissingAndApproved(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureOutput: map[string]string{},
		captureErr: map[string]error{
			"restic version": errors.New("not found"),
		},
		runErr: map[string]error{},
	}

	err := SyncInteractive(context.Background(), config.File{
		ResticVersion: "0.18.1",
		Profiles:      map[string]config.Profile{"wsl": {Repository: "/repo/wsl"}},
	}, exec, func(string) (bool, error) {
		return true, nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(exec.runCalls) != 1 {
		t.Fatalf("expected one install call, got %v", exec.runCalls)
	}
	if exec.runCalls[0] != "sudo dnf install -y restic" {
		t.Fatalf("unexpected install command: %s", exec.runCalls[0])
	}
}

func TestSyncInteractiveWithReportReturnsWSLMatchedStatus(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureOutput: map[string]string{
			"restic version": "restic 0.18.1 compiled with go1.24",
		},
	}

	report, err := SyncInteractiveWithReport(context.Background(), config.File{
		ResticVersion: "0.18.1",
		Profiles:      map[string]config.Profile{"wsl": {Repository: "/repo/wsl"}},
	}, exec, func(string) (bool, error) { return true, nil })

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("expected 1 report item, got %d", len(report.Items))
	}
	if report.Items[0].Status != SetupMatched {
		t.Fatalf("expected matched status, got %s", report.Items[0].Status)
	}
}

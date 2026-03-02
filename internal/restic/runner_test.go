package restic

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestOSRunnerRunsWithPasswordFromEnv(t *testing.T) {
	original := commandContext
	commandContext = fakeExecCommand
	t.Cleanup(func() { commandContext = original })

	t.Setenv("RESTIC_PASSWORD", "test-password")

	var stdout strings.Builder
	var stderr strings.Builder
	runner := NewOSRunner(&stdout, &stderr)

	err := runner.Run(context.Background(), "snapshots")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(stdout.String(), "$ restic snapshots") {
		t.Fatalf("expected command echo in stdout, got %q", stdout.String())
	}
}

func TestOSRunnerFailsWithoutPasswordSource(t *testing.T) {
	original := commandContext
	commandContext = fakeExecCommand
	t.Cleanup(func() { commandContext = original })

	t.Setenv("RESTIC_PASSWORD", "")
	t.Setenv(WSLBackupPasswordFileEnv, "")
	t.Setenv(ResticPasswordFileEnv, "")
	t.Setenv(SystemdCredentialsDirEnv, "")

	var stdout strings.Builder
	var stderr strings.Builder
	runner := NewOSRunner(&stdout, &stderr)

	err := runner.Run(context.Background(), "snapshots")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "restic password is not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithResticPasswordReplacesExistingValue(t *testing.T) {
	base := []string{"PATH=/usr/bin", "RESTIC_PASSWORD=old", "HOME=/tmp"}
	updated := withResticPassword(base, "new-secret")

	if strings.Count(strings.Join(updated, "|"), "RESTIC_PASSWORD=") != 1 {
		t.Fatalf("expected exactly one RESTIC_PASSWORD entry, got %#v", updated)
	}
	if updated[len(updated)-1] != "RESTIC_PASSWORD=new-secret" {
		t.Fatalf("expected appended replacement password, got %#v", updated)
	}
}

func fakeExecCommand(_ context.Context, _ string, _ ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	if os.Getenv("RESTIC_PASSWORD") == "" {
		fmt.Fprintln(os.Stderr, "missing RESTIC_PASSWORD")
		os.Exit(2)
	}

	fmt.Fprintln(os.Stdout, "helper stdout")
	fmt.Fprintln(os.Stderr, "helper stderr")
	os.Exit(0)
}

package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestOSExecutorRunCaptureEchoesAndCapturesOutput(t *testing.T) {
	original := commandContext
	commandContext = fakeExecCommand
	t.Cleanup(func() {
		commandContext = original
	})

	var stdout strings.Builder
	var stderr strings.Builder
	exec := NewOSExecutor(&stdout, &stderr)

	output, err := exec.RunCapture(context.Background(), "restic", "version")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(stdout.String(), "$ restic version") {
		t.Fatalf("expected command echo in stdout")
	}
	if !strings.Contains(output, "helper stderr") {
		t.Fatalf("expected captured stderr, got %q", output)
	}
	if strings.TrimSpace(output) == "" {
		t.Fatalf("expected captured command output")
	}
}

func TestOSExecutorRunEchoesCommandWithoutArgs(t *testing.T) {
	original := commandContext
	commandContext = fakeExecCommand
	t.Cleanup(func() {
		commandContext = original
	})

	var stdout strings.Builder
	var stderr strings.Builder
	exec := NewOSExecutor(&stdout, &stderr)

	err := exec.Run(context.Background(), "restic")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(stdout.String(), "$ restic") {
		t.Fatalf("expected echoed command without trailing arg text")
	}
}

func TestOSExecutorRunWithEnvEchoesCommand(t *testing.T) {
	original := commandContext
	commandContext = fakeExecCommand
	t.Cleanup(func() {
		commandContext = original
	})

	var stdout strings.Builder
	var stderr strings.Builder
	exec := NewOSExecutor(&stdout, &stderr)

	err := exec.RunWithEnv(context.Background(), map[string]string{"RESTIC_PASSWORD": "secret"}, "restic", "snapshots")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(stdout.String(), "$ restic snapshots") {
		t.Fatalf("expected echoed command")
	}
}

func TestFormatCommandQuotesWhitespaceArgs(t *testing.T) {
	formatted := formatCommand([]string{"backup", "--target", "/tmp/my folder"})
	if formatted != "backup --target \"/tmp/my folder\"" {
		t.Fatalf("unexpected formatted command: %q", formatted)
	}
}

func TestMergeEnvReplacesExistingKeys(t *testing.T) {
	merged := mergeEnv([]string{"A=1", "B=2"}, map[string]string{"B": "9", "C": "3"})
	joined := strings.Join(merged, "|")
	if strings.Contains(joined, "B=2") {
		t.Fatalf("expected old B value to be replaced, got %v", merged)
	}
	if !strings.Contains(joined, "B=9") {
		t.Fatalf("expected new B value, got %v", merged)
	}
	if !strings.Contains(joined, "C=3") {
		t.Fatalf("expected C to be added, got %v", merged)
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

	fmt.Fprintln(os.Stdout, "helper stdout")
	fmt.Fprintln(os.Stderr, "helper stderr")
	os.Exit(0)
}

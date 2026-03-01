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
	if !strings.Contains(output, "helper stdout") {
		t.Fatalf("expected captured stdout, got %q", output)
	}
	if !strings.Contains(output, "helper stderr") {
		t.Fatalf("expected captured stderr, got %q", output)
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

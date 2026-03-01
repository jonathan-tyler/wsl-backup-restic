package restic

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestFormatCommandQuotesWhitespace(t *testing.T) {
	formatted := formatCommand([]string{"backup", "--target", "/tmp/my folder"})
	if formatted != "backup --target \"/tmp/my folder\"" {
		t.Fatalf("unexpected format: %s", formatted)
	}
}

func TestOSRunnerPrintsCommandAndStreamsOutput(t *testing.T) {
	original := commandContext
	commandContext = fakeExecCommand
	t.Cleanup(func() {
		commandContext = original
	})

	var stdout strings.Builder
	var stderr strings.Builder
	runner := NewOSRunner(&stdout, &stderr)

	err := runner.Run(context.Background(), "snapshots")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "$ restic snapshots") {
		t.Fatalf("expected echoed command in stdout, got %q", out)
	}
	if !strings.Contains(out, "helper stdout") {
		t.Fatalf("expected command stdout in stdout writer, got %q", out)
	}
	if !strings.Contains(stderr.String(), "helper stderr") {
		t.Fatalf("expected command stderr in stderr writer, got %q", stderr.String())
	}
}

func TestOSRunnerRejectsEmptyArgs(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	runner := NewOSRunner(&stdout, &stderr)
	err := runner.Run(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
}

func fakeExecCommand(_ context.Context, _ string, args ...string) *exec.Cmd {
	allArgs := []string{"-test.run=TestHelperProcess", "--"}
	allArgs = append(allArgs, args...)
	cmd := exec.Command(os.Args[0], allArgs...)
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

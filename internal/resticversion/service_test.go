package resticversion

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/example/wsl-backup/internal/config"
)

type fakeSystemExecutor struct {
	captureOutput map[string]string
	captureErr    map[string]error
	runErr        map[string]error

	runCalls []string
}

func (e *fakeSystemExecutor) Run(_ context.Context, name string, args ...string) error {
	key := buildCmdKey(name, args...)
	e.runCalls = append(e.runCalls, key)
	if err, ok := e.runErr[key]; ok {
		return err
	}
	return nil
}

func (e *fakeSystemExecutor) RunCapture(_ context.Context, name string, args ...string) (string, error) {
	key := buildCmdKey(name, args...)
	if out, ok := e.captureOutput[key]; ok {
		if err, hasErr := e.captureErr[key]; hasErr {
			return out, err
		}
		return out, nil
	}
	if err, ok := e.captureErr[key]; ok {
		return "", err
	}
	return "", nil
}

func buildCmdKey(name string, args ...string) string {
	return name + " " + strings.Join(args, " ")
}

func TestCheckCompatibleNoActionWhenMatching(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureOutput: map[string]string{
			"restic version":                             "restic 0.18.1 compiled with go1.24",
			"powershell.exe -NoProfile -Command restic version": "restic 0.18.1 compiled with go1.24",
		},
		captureErr: map[string]error{},
		runErr:     map[string]error{},
	}

	err := CheckCompatible(context.Background(), config.File{
		ResticVersion: "0.18.1",
		Profiles: map[string]config.Profile{
			"wsl":     {Repository: "/repo/wsl"},
			"windows": {Repository: `C:\repo`},
		},
	}, exec)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(exec.runCalls) != 0 {
		t.Fatalf("expected no install/update calls, got %v", exec.runCalls)
	}
}

func TestCheckCompatibleFailsWithSetupHintOnMismatch(t *testing.T) {
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

func TestSyncInteractiveInstallsWhenMissingAndApproved(t *testing.T) {
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

func TestParseResticVersion(t *testing.T) {
	version, err := parseResticVersion("restic 0.18.1 compiled with go1.24")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if version != "0.18.1" {
		t.Fatalf("unexpected version: %s", version)
	}
}

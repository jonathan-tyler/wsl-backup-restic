package resticversion

import (
	"context"
	"testing"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
)

func TestCheckCompatibleNoActionWhenMatching(t *testing.T) {
	exec := &fakeSystemExecutor{
		captureOutput: map[string]string{
			"restic version":                             "restic 0.18.1 compiled with go1.24",
			"pwsh.exe -NoProfile -Command restic version": "restic 0.18.1 compiled with go1.24",
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

func TestParseResticVersion(t *testing.T) {
	version, err := parseResticVersion("restic 0.18.1 compiled with go1.24")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if version != "0.18.1" {
		t.Fatalf("unexpected version: %s", version)
	}
}

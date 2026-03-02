package setup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/apperr"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/resticversion"
)

type fakeLoader struct {
	cfg config.File
	err error
}

func (l fakeLoader) Load() (config.File, error) {
	if l.err != nil {
		return config.File{}, l.err
	}
	return l.cfg, nil
}

type fakeSystem struct {
	runCaptureOutput string
	runCaptureErr    error
	runErr           error
}

func (s fakeSystem) Run(_ context.Context, _ string, _ ...string) error {
	return s.runErr
}

func (s fakeSystem) RunWithEnv(_ context.Context, _ map[string]string, _ string, _ ...string) error {
	return s.runErr
}

func (s fakeSystem) RunCapture(_ context.Context, _ string, _ ...string) (string, error) {
	return s.runCaptureOutput, s.runCaptureErr
}

func TestHandleRejectsPositionalArgs(t *testing.T) {
	err := Handle(context.Background(), []string{"extra"}, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, ok := err.(apperr.UsageError); !ok {
		t.Fatalf("expected usage error, got %T", err)
	}
}

func TestHandleWithRunsSyncAndPasses(t *testing.T) {
	deps := Dependencies{
		Loader: fakeLoader{cfg: config.File{
			ResticVersion: "0.18.1",
			Profiles: map[string]config.Profile{
				"wsl": {Repository: "/repo/wsl"},
			},
		}},
		System: fakeSystem{runCaptureOutput: "restic 0.18.1 compiled with go"},
		Confirm: func(string) (bool, error) {
			return true, nil
		},
	}

	err := HandleWith(context.Background(), deps)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestHandleWithReturnsLoaderError(t *testing.T) {
	err := HandleWith(context.Background(), Dependencies{Loader: fakeLoader{err: fmt.Errorf("load fail")}})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestHandlePrintsSuccessSummary(t *testing.T) {
	originalStdout := setupStdout
	originalStderr := setupStderr
	originalStdin := setupStdin
	originalLoader := newConfigLoader
	originalSystem := newSystemExecutor
	originalConfirm := newConfirmFunc
	originalHandleWithReport := handleWithReportFunc
	t.Cleanup(func() {
		setupStdout = originalStdout
		setupStderr = originalStderr
		setupStdin = originalStdin
		newConfigLoader = originalLoader
		newSystemExecutor = originalSystem
		newConfirmFunc = originalConfirm
		handleWithReportFunc = originalHandleWithReport
	})

	var output strings.Builder
	setupStdout = &output
	setupStderr = io.Discard
	setupStdin = strings.NewReader("")
	handleWithReportFunc = func(context.Context, Dependencies) (resticversion.SetupReport, error) {
		return resticversion.SetupReport{
			Items: []resticversion.PlatformReport{{Platform: "wsl", Status: resticversion.SetupMatched, Message: "ok"}},
		}, nil
	}

	err := Handle(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	text := output.String()
	if !strings.Contains(text, "Running backup setup checks and installers") {
		t.Fatalf("expected startup message, got %q", text)
	}
	if !strings.Contains(text, "setup report:") {
		t.Fatalf("expected setup report, got %q", text)
	}
	if !strings.Contains(text, "Backup setup completed successfully") {
		t.Fatalf("expected success summary, got %q", text)
	}
}

func TestHandlePrintsFailureSummary(t *testing.T) {
	originalStdout := setupStdout
	originalStderr := setupStderr
	originalStdin := setupStdin
	originalLoader := newConfigLoader
	originalSystem := newSystemExecutor
	originalConfirm := newConfirmFunc
	originalHandleWithReport := handleWithReportFunc
	t.Cleanup(func() {
		setupStdout = originalStdout
		setupStderr = originalStderr
		setupStdin = originalStdin
		newConfigLoader = originalLoader
		newSystemExecutor = originalSystem
		newConfirmFunc = originalConfirm
		handleWithReportFunc = originalHandleWithReport
	})

	var output strings.Builder
	setupStdout = &output
	setupStderr = io.Discard
	setupStdin = strings.NewReader("")
	handleWithReportFunc = func(context.Context, Dependencies) (resticversion.SetupReport, error) {
		return resticversion.SetupReport{}, fmt.Errorf("sync failed")
	}

	err := Handle(context.Background(), nil, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "sync failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output.String(), "Backup setup failed: sync failed") {
		t.Fatalf("expected failure summary, got %q", output.String())
	}
}

func TestHandleWithReportDefaultsAllDependencies(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`restic_version: ""
profiles:
  wsl:
    repository: /repo/wsl
    use_fs_snapshot: false
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("BACKUP_CONFIG", configPath)

	originalStdout := setupStdout
	originalStderr := setupStderr
	t.Cleanup(func() {
		setupStdout = originalStdout
		setupStderr = originalStderr
	})
	setupStdout = io.Discard
	setupStderr = io.Discard

	report, err := handleWithReport(context.Background(), Dependencies{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("expected one report item, got %d", len(report.Items))
	}
	if report.Items[0].Status != resticversion.SetupSkipped {
		t.Fatalf("expected skipped status, got %s", report.Items[0].Status)
	}
}

func TestHandleWithUsesDefaultConfirmWhenNil(t *testing.T) {
	deps := Dependencies{
		Loader: fakeLoader{cfg: config.File{
			ResticVersion: "",
			Profiles: map[string]config.Profile{
				"wsl": {Repository: "/repo/wsl"},
			},
		}},
		System: fakeSystem{},
	}

	err := HandleWith(context.Background(), deps)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestPrintSetupReportNoItems(t *testing.T) {
	originalStdout := setupStdout
	t.Cleanup(func() {
		setupStdout = originalStdout
	})

	var output strings.Builder
	setupStdout = &output
	printSetupReport(resticversion.SetupReport{})

	if !strings.Contains(output.String(), "no profile checks were executed") {
		t.Fatalf("expected no-items report message, got %q", output.String())
	}
}

func TestPrintSetupReportWithItems(t *testing.T) {
	report := resticversion.SetupReport{
		Items: []resticversion.PlatformReport{
			{Platform: "wsl", Status: resticversion.SetupMatched, Message: "ok"},
			{Platform: "windows", Status: resticversion.SetupUpdated, Message: "updated via scoop"},
		},
	}

	originalStdout := setupStdout
	t.Cleanup(func() {
		setupStdout = originalStdout
	})

	var output strings.Builder
	setupStdout = &output
	printSetupReport(report)

	if !strings.Contains(output.String(), "setup report:") {
		t.Fatalf("expected report header, got %q", output.String())
	}
	if !strings.Contains(output.String(), "- wsl: matched (ok)") {
		t.Fatalf("expected wsl report line, got %q", output.String())
	}
	if !strings.Contains(output.String(), "- windows: updated (updated via scoop)") {
		t.Fatalf("expected windows report line, got %q", output.String())
	}
}

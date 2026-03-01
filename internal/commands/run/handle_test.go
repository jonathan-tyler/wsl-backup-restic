package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/example/wsl-backup/internal/apperr"
	"github.com/example/wsl-backup/internal/config"
)

type fakeRunner struct {
	calls [][]string
	err   error
}

func (f *fakeRunner) Run(_ context.Context, args ...string) error {
	f.calls = append(f.calls, append([]string{}, args...))
	return f.err
}

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

func withTempRules(t *testing.T, cadence string, includeProfiles []string, excludeProfiles []string) string {
	t.Helper()

	dir := t.TempDir()
	for _, profile := range includeProfiles {
		path := filepath.Join(dir, fmt.Sprintf("%s.include.%s.txt", profile, cadence))
		if err := os.WriteFile(path, []byte("/tmp/source\n"), 0o644); err != nil {
			t.Fatalf("write include rules: %v", err)
		}
	}

	for _, profile := range excludeProfiles {
		path := filepath.Join(dir, fmt.Sprintf("%s.exclude.%s.txt", profile, cadence))
		if err := os.WriteFile(path, []byte("*.tmp\n"), 0o644); err != nil {
			t.Fatalf("write exclude rules: %v", err)
		}
	}

	return dir
}

func TestHandleRequiresCadence(t *testing.T) {
	err := HandleWith(context.Background(), nil, &fakeRunner{}, RunDependencies{Loader: fakeLoader{}, Stat: os.Stat})
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, ok := err.(apperr.UsageError); !ok {
		t.Fatalf("expected usage error, got %T", err)
	}
}

func TestHandleRejectsUnknownCadence(t *testing.T) {
	err := HandleWith(context.Background(), []string{"yearly"}, &fakeRunner{}, RunDependencies{Loader: fakeLoader{}, Stat: os.Stat})
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, ok := err.(apperr.UsageError); !ok {
		t.Fatalf("expected usage error, got %T", err)
	}
}

func TestHandleRunsConfiguredProfiles(t *testing.T) {
	rulesDir := withTempRules(t, "weekly", []string{"wsl", "windows"}, []string{"windows"})
	runner := &fakeRunner{}
	loader := fakeLoader{cfg: config.File{
		Profiles: map[string]config.Profile{
			"windows": {Repository: `C:\repo`, UseFSSnapshot: true},
			"wsl":     {Repository: "/repo/wsl", UseFSSnapshot: false},
		},
	}}
	loader.cfgPathSetForTest(rulesDir)

	err := HandleWith(context.Background(), []string{"weekly", "--one-file-system"}, runner, RunDependencies{Loader: loader, Stat: os.Stat})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 profile runs, got %d", len(runner.calls))
	}

	hasWindows := false
	hasWSL := false
	for _, call := range runner.calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, `C:\repo`) {
			hasWindows = true
			if !strings.Contains(joined, "--use-fs-snapshot") {
				t.Fatalf("expected windows profile args to include --use-fs-snapshot: %v", call)
			}
			if !strings.Contains(joined, "windows.include.weekly.txt") {
				t.Fatalf("expected windows include file arg: %v", call)
			}
			if !strings.Contains(joined, "windows.exclude.weekly.txt") {
				t.Fatalf("expected windows exclude file arg: %v", call)
			}
		}

		if strings.Contains(joined, "/repo/wsl") {
			hasWSL = true
			if strings.Contains(joined, "--use-fs-snapshot") {
				t.Fatalf("did not expect wsl profile args to include --use-fs-snapshot: %v", call)
			}
			if !strings.Contains(joined, "wsl.include.weekly.txt") {
				t.Fatalf("expected wsl include file arg: %v", call)
			}
		}

		if !strings.Contains(joined, "--one-file-system") {
			t.Fatalf("expected passthrough restic args: %v", call)
		}
	}

	if !hasWindows || !hasWSL {
		t.Fatalf("expected both windows and wsl profiles to run")
	}
}

func TestHandleFailsWhenIncludeRulesMissing(t *testing.T) {
	rulesDir := withTempRules(t, "daily", []string{}, []string{})
	runner := &fakeRunner{}
	loader := fakeLoader{cfg: config.File{Profiles: map[string]config.Profile{"wsl": {Repository: "/repo/wsl"}}}}
	loader.cfgPathSetForTest(rulesDir)

	err := HandleWith(context.Background(), []string{"daily"}, runner, RunDependencies{Loader: loader, Stat: os.Stat})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "missing include rules file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleFailsWhenLoaderFails(t *testing.T) {
	err := HandleWith(context.Background(), []string{"daily"}, &fakeRunner{}, RunDependencies{Loader: fakeLoader{err: fmt.Errorf("load fail")}, Stat: os.Stat})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "load fail") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func (l *fakeLoader) cfgPathSetForTest(dir string) {
	l.cfg = config.FileWithPathForTest(l.cfg, filepath.Join(dir, "config.yaml"))
}

func TestBuildRunArgsErrorsOnUnexpectedExcludeStatFailure(t *testing.T) {
	_, err := buildRunArgs("/cfg", "wsl", config.Profile{Repository: "/repo"}, "daily", nil, func(path string) (os.FileInfo, error) {
		if strings.Contains(path, ".include.") {
			return fakeFileInfo{}, nil
		}
		return nil, fmt.Errorf("permission denied")
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "exclude rules check failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "x" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() os.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() interface{}   { return nil }

package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/apperr"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
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

type fakeSystem struct {
	runCalls    [][]string
	runWithEnv  []map[string]string
	runCapture  map[string]string
	runErr      map[string]error
	captureErr  map[string]error
}

func (s *fakeSystem) Run(_ context.Context, name string, args ...string) error {
	call := append([]string{name}, args...)
	s.runCalls = append(s.runCalls, call)
	s.runWithEnv = append(s.runWithEnv, map[string]string{})
	if s.runErr != nil {
		if err, ok := s.runErr[strings.Join(call, " ")]; ok {
			return err
		}
	}
	return nil
}

func (s *fakeSystem) RunWithEnv(_ context.Context, env map[string]string, name string, args ...string) error {
	call := append([]string{name}, args...)
	s.runCalls = append(s.runCalls, call)
	envCopy := map[string]string{}
	for key, value := range env {
		envCopy[key] = value
	}
	s.runWithEnv = append(s.runWithEnv, envCopy)
	if s.runErr != nil {
		if err, ok := s.runErr[strings.Join(call, " ")]; ok {
			return err
		}
	}
	return nil
}

func (s *fakeSystem) RunCapture(_ context.Context, name string, args ...string) (string, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	if s.captureErr != nil {
		if err, ok := s.captureErr[key]; ok {
			return "", err
		}
	}
	if s.runCapture != nil {
		if out, ok := s.runCapture[key]; ok {
			return out, nil
		}
	}
	return "", nil
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
		if err := os.WriteFile(path, []byte("/tmp/source-"+profile+"\n"), 0o644); err != nil {
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

func withTempRepository(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("repo-config"), 0o644); err != nil {
		t.Fatalf("write repo config: %v", err)
	}

	return dir
}

func withTempKeepassDB(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "vault.kdbx")
	if err := os.WriteFile(path, []byte("db"), 0o644); err != nil {
		t.Fatalf("write keepass db: %v", err)
	}

	return path
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
	t.Setenv("RESTIC_PASSWORD", "test-password")

	rulesDir := withTempRules(t, "weekly", []string{"wsl", "windows"}, []string{"wsl", "windows"})
	wslRepo := withTempRepository(t)
	windowsRepo := withTempRepository(t)
	keepassDB := withTempKeepassDB(t)
	runner := &fakeRunner{}
	fakeExec := &fakeSystem{
		runCapture: map[string]string{},
	}
	loader := fakeLoader{cfg: config.File{
		ResticVersion: "0.18.1",
		KeepassDB:     keepassDB,
		Profiles: map[string]config.Profile{
			"windows": {Repository: windowsRepo, UseFSSnapshot: true},
			"wsl":     {Repository: wslRepo, UseFSSnapshot: false},
		},
	}}
	loader.cfgPathSetForTest(rulesDir)
	fakeExec.runCapture["restic version"] = "restic 0.18.1 compiled with go"
	fakeExec.runCapture["powershell.exe -NoProfile -Command restic version"] = "restic 0.18.1 compiled with go"
	fakeExec.runCapture["wslpath -w "+filepath.Join(rulesDir, "windows.include.weekly.txt")] = "C:\\rules\\windows.include.weekly.txt"
	fakeExec.runCapture["wslpath -w "+filepath.Join(rulesDir, "windows.exclude.weekly.txt")] = "C:\\rules\\windows.exclude.weekly.txt"

	err := HandleWith(context.Background(), []string{"weekly", "--one-file-system"}, runner, RunDependencies{Loader: loader, Stat: os.Stat, System: fakeExec})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 wsl profile run via restic runner, got %d", len(runner.calls))
	}

	if len(fakeExec.runCalls) != 1 {
		t.Fatalf("expected 1 windows profile run via system executor, got %d", len(fakeExec.runCalls))
	}

	hasWSL := false
	for _, call := range runner.calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, wslRepo) {
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

	windowsCall := strings.Join(fakeExec.runCalls[0], " ")
	if !strings.Contains(windowsCall, "restic.exe") {
		t.Fatalf("expected windows execution with restic.exe, got %v", fakeExec.runCalls[0])
	}
	if !strings.Contains(windowsCall, windowsRepo) {
		t.Fatalf("expected windows repo path in call, got %v", fakeExec.runCalls[0])
	}
	if !strings.Contains(windowsCall, "--use-fs-snapshot") {
		t.Fatalf("expected windows fs snapshot flag in call, got %v", fakeExec.runCalls[0])
	}
	if !strings.Contains(windowsCall, `C:\rules\windows.include.weekly.txt`) {
		t.Fatalf("expected converted include path in windows call, got %v", fakeExec.runCalls[0])
	}
	if !strings.Contains(windowsCall, `C:\rules\windows.exclude.weekly.txt`) {
		t.Fatalf("expected converted exclude path in windows call, got %v", fakeExec.runCalls[0])
	}

	if !hasWSL {
		t.Fatalf("expected wsl profile to run")
	}
}

func TestHandleFailsWhenIncludeRulesMissing(t *testing.T) {
	rulesDir := withTempRules(t, "daily", []string{}, []string{})
	wslRepo := withTempRepository(t)
	keepassDB := withTempKeepassDB(t)
	runner := &fakeRunner{}
	loader := fakeLoader{cfg: config.File{
		KeepassDB: keepassDB,
		Profiles:  map[string]config.Profile{"wsl": {Repository: wslRepo}},
	}}
	loader.cfgPathSetForTest(rulesDir)

	err := HandleWith(context.Background(), []string{"daily"}, runner, RunDependencies{Loader: loader, Stat: os.Stat})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "missing include rules file") && !strings.Contains(err.Error(), "read include rules failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleFailsWhenKeepassDatabaseMissing(t *testing.T) {
	rulesDir := withTempRules(t, "daily", []string{"wsl"}, []string{"wsl"})
	wslRepo := withTempRepository(t)
	runner := &fakeRunner{}
	loader := fakeLoader{cfg: config.File{
		KeepassDB: filepath.Join(t.TempDir(), "missing.kdbx"),
		Profiles:  map[string]config.Profile{"wsl": {Repository: wslRepo}},
	}}
	loader.cfgPathSetForTest(rulesDir)

	err := HandleWith(context.Background(), []string{"daily"}, runner, RunDependencies{Loader: loader, Stat: os.Stat})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "keepassxc database not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleOffersRepositoryCreation(t *testing.T) {
	rulesDir := withTempRules(t, "daily", []string{"wsl"}, []string{"wsl"})
	keepassDB := withTempKeepassDB(t)
	runner := &fakeRunner{}
	loader := fakeLoader{cfg: config.File{
		KeepassDB: keepassDB,
		Profiles: map[string]config.Profile{
			"wsl": {Repository: filepath.Join(t.TempDir(), "missing-repo")},
		},
	}}
	loader.cfgPathSetForTest(rulesDir)

	confirmed := false
	err := HandleWith(context.Background(), []string{"daily"}, runner, RunDependencies{
		Loader: loader,
		Stat:   os.Stat,
		Confirm: func(_ string) (bool, error) {
			confirmed = true
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !confirmed {
		t.Fatalf("expected repository creation prompt")
	}
	if len(runner.calls) == 0 {
		t.Fatalf("expected restic runner calls")
	}
	initCall := strings.Join(runner.calls[0], " ")
	if !strings.Contains(initCall, "init --repo") {
		t.Fatalf("expected first runner call to initialize repository, got %v", runner.calls[0])
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

func TestValidateIncludeRuleOverlapFailsOnCrossProfileOverlap(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wsl.include.daily.txt"), []byte("/data/projects\n"), 0o644); err != nil {
		t.Fatalf("write include rules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "windows.include.daily.txt"), []byte("/data/projects/app\n"), 0o644); err != nil {
		t.Fatalf("write include rules: %v", err)
	}

	err := validateIncludeRuleOverlap(dir, "daily", map[string]config.Profile{
		"wsl":     {Repository: "/repo/wsl"},
		"windows": {Repository: `C:\repo`},
	}, os.ReadFile)
	if err == nil {
		t.Fatalf("expected overlap error")
	}
	if !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateIncludeRuleOverlapAllowsDistinctPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wsl.include.daily.txt"), []byte("/data/projects\n"), 0o644); err != nil {
		t.Fatalf("write include rules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "windows.include.daily.txt"), []byte("/mnt/c/Users/me/documents\n"), 0o644); err != nil {
		t.Fatalf("write include rules: %v", err)
	}

	err := validateIncludeRuleOverlap(dir, "daily", map[string]config.Profile{
		"wsl":     {Repository: "/repo/wsl"},
		"windows": {Repository: `C:\repo`},
	}, os.ReadFile)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateIncludeRuleOverlapAllowsExcludeOverlap(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wsl.include.daily.txt"), []byte("/data/wsl\n"), 0o644); err != nil {
		t.Fatalf("write include rules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "windows.include.daily.txt"), []byte("/data/windows\n"), 0o644); err != nil {
		t.Fatalf("write include rules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wsl.exclude.daily.txt"), []byte("/mnt/c/Users/mike\n"), 0o644); err != nil {
		t.Fatalf("write exclude rules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "windows.exclude.daily.txt"), []byte("C:\\Users\\mike\\docs\n"), 0o644); err != nil {
		t.Fatalf("write exclude rules: %v", err)
	}

	err := validateIncludeRuleOverlap(dir, "daily", map[string]config.Profile{
		"wsl":     {Repository: "/repo/wsl"},
		"windows": {Repository: `C:\repo`},
	}, os.ReadFile)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateIncludeRuleOverlapDetectsWSLAndWindowsEquivalentPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wsl.include.daily.txt"), []byte("/mnt/c/Users/mike/file.txt\n"), 0o644); err != nil {
		t.Fatalf("write include rules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "windows.include.daily.txt"), []byte("C:\\Users\\mike\\file.txt\n"), 0o644); err != nil {
		t.Fatalf("write include rules: %v", err)
	}

	err := validateIncludeRuleOverlap(dir, "daily", map[string]config.Profile{
		"wsl":     {Repository: "/repo/wsl"},
		"windows": {Repository: `C:\repo`},
	}, os.ReadFile)
	if err == nil {
		t.Fatalf("expected overlap error")
	}
	if !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizePathConvertsWSLMountDrivePath(t *testing.T) {
	normalized := normalizePath("/mnt/C/Users/mike/file.txt")
	if normalized != "c:/users/mike/file.txt" {
		t.Fatalf("unexpected normalized path: %s", normalized)
	}
}

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "x" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() os.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() interface{}   { return nil }

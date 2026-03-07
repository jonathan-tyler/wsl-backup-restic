package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/apperr"
	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/config"
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
	if name == "wslpath" && len(args) == 2 && args[0] == "-w" {
		if out, ok := s.runCapture[key]; ok {
			return out, nil
		}
		return "C:\\converted\\path", nil
	}
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
	cadences, err := inheritedCadences(cadence)
	if err != nil {
		t.Fatalf("resolve cadences: %v", err)
	}

	for _, item := range cadences {
		if len(includeProfiles) > 0 {
			path := filepath.Join(dir, fmt.Sprintf("includes.%s.txt", item))
			if err := os.WriteFile(path, []byte("/tmp/source\n"), 0o644); err != nil {
				t.Fatalf("write include rules: %v", err)
			}
		}
	}

	if len(excludeProfiles) > 0 {
		path := filepath.Join(dir, "excludes.txt")
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
	runner := &fakeRunner{}
	fakeExec := &fakeSystem{
		runCapture: map[string]string{},
	}
	loader := fakeLoader{cfg: config.File{
		ResticVersion: "0.18.1",
		Profiles: map[string]config.Profile{
			"windows": {Repository: windowsRepo, UseFSSnapshot: false},
			"wsl":     {Repository: wslRepo, UseFSSnapshot: false},
		},
	}}
	loader.cfgPathSetForTest(rulesDir)
	fakeExec.runCapture["restic version"] = "restic 0.18.1 compiled with go"
	fakeExec.runCapture["pwsh.exe -NoProfile -Command restic version"] = "restic 0.18.1 compiled with go"
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-001.txt")] = "C:\\rules\\includes.weekly.txt"
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-002.txt")] = "C:\\rules\\includes.daily.txt"
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-003.txt")] = "C:\\rules\\excludes.txt"
	fakeExec.runCapture["wslpath -w "+filepath.Join(rulesDir, "excludes.txt")] = "C:\\rules\\excludes.txt"
	originalCreateTemp := osCreateTemp
	createTempIndex := 0
	osCreateTemp = func(dir string, pattern string) (*os.File, error) {
		createTempIndex++
		name := fmt.Sprintf("wsl-backup-orchestrator-rule-%03d.txt", createTempIndex)
		if strings.Contains(pattern, "password") {
			name = "wsl-backup-orchestrator-password-001.txt"
		}
		path := filepath.Join(os.TempDir(), name)
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
	t.Cleanup(func() {
		osCreateTemp = originalCreateTemp
		_ = os.Remove(filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-001.txt"))
		_ = os.Remove(filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-002.txt"))
		_ = os.Remove(filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-003.txt"))
		_ = os.Remove(filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-001.txt"))
	})
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-001.txt")] = "C:\\rules\\backup-password.txt"

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
			if !strings.Contains(joined, "includes.weekly.txt") {
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
	if !strings.Contains(windowsCall, `C:\rules\includes.weekly.txt`) {
		t.Fatalf("expected converted include path in windows call, got %v", fakeExec.runCalls[0])
	}
	if !strings.Contains(windowsCall, `C:\rules\excludes.txt`) {
		t.Fatalf("expected converted exclude path in windows call, got %v", fakeExec.runCalls[0])
	}

	if !hasWSL {
		t.Fatalf("expected wsl profile to run")
	}
}

func TestHandleFailsWhenIncludeRulesMissing(t *testing.T) {
	rulesDir := withTempRules(t, "daily", []string{}, []string{})
	wslRepo := withTempRepository(t)
	runner := &fakeRunner{}
	loader := fakeLoader{cfg: config.File{
		Profiles: map[string]config.Profile{"wsl": {Repository: wslRepo}},
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

func TestHandleFailsWhenPasswordMissing(t *testing.T) {
	rulesDir := withTempRules(t, "daily", []string{"wsl"}, []string{"wsl"})
	wslRepo := withTempRepository(t)
	runner := &fakeRunner{}
	loader := fakeLoader{cfg: config.File{
		Profiles: map[string]config.Profile{"wsl": {Repository: wslRepo}},
	}}
	loader.cfgPathSetForTest(rulesDir)
	t.Setenv("RESTIC_PASSWORD", "")

	err := HandleWith(context.Background(), []string{"daily"}, runner, RunDependencies{Loader: loader, Stat: os.Stat})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "restic password is not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleFailsWhenProfilesShareNormalizedRepository(t *testing.T) {
	t.Setenv("RESTIC_PASSWORD", "test-password")

	rulesDir := withTempRules(t, "daily", []string{"wsl", "windows"}, []string{"wsl", "windows"})
	runner := &fakeRunner{}
	loader := fakeLoader{cfg: config.File{
		Profiles: map[string]config.Profile{
			"wsl":     {Repository: "/mnt/c/backups/shared"},
			"windows": {Repository: `C:\backups\shared`},
		},
	}}
	loader.cfgPathSetForTest(rulesDir)

	err := HandleWith(context.Background(), []string{"daily"}, runner, RunDependencies{Loader: loader, Stat: os.Stat})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "target the same repository after normalization") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleOffersRepositoryCreation(t *testing.T) {
	t.Setenv("RESTIC_PASSWORD", "test-password")

	rulesDir := withTempRules(t, "daily", []string{"wsl"}, []string{"wsl"})
	runner := &fakeRunner{}
	loader := fakeLoader{cfg: config.File{
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

func TestHandleOffersWindowsRepositoryCreationWithPasswordFile(t *testing.T) {
	t.Setenv("RESTIC_PASSWORD", "test-password")

	rulesDir := withTempRules(t, "daily", []string{"windows"}, []string{"windows"})
	runner := &fakeRunner{}
	fakeExec := &fakeSystem{runCapture: map[string]string{}}
	loader := fakeLoader{cfg: config.File{
		ResticVersion: "0.18.1",
		Profiles: map[string]config.Profile{
			"windows": {Repository: `C:\\missing\\repo`, UseFSSnapshot: false},
		},
	}}
	loader.cfgPathSetForTest(rulesDir)
	fakeExec.runCapture["restic version"] = "restic 0.18.1 compiled with go"
	fakeExec.runCapture["pwsh.exe -NoProfile -Command restic version"] = "restic 0.18.1 compiled with go"
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-001.txt")] = `C:\\rules\\includes.daily.txt`
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-002.txt")] = `C:\\rules\\excludes.txt`
	originalCreateTemp := osCreateTemp
	createTempIndex := 0
	osCreateTemp = func(dir string, pattern string) (*os.File, error) {
		createTempIndex++
		name := fmt.Sprintf("wsl-backup-orchestrator-rule-%03d.txt", createTempIndex)
		if strings.Contains(pattern, "password") {
			name = "wsl-backup-orchestrator-password-001.txt"
		}
		path := filepath.Join(os.TempDir(), name)
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
	t.Cleanup(func() {
		osCreateTemp = originalCreateTemp
		_ = os.Remove(filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-001.txt"))
		_ = os.Remove(filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-002.txt"))
		_ = os.Remove(filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-001.txt"))
	})
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-001.txt")] = `C:\\rules\\backup-password.txt`
	fakeExec.runCapture["wslpath -w "+filepath.Join(rulesDir, "excludes.txt")] = `C:\\rules\\excludes.txt`

	confirmed := false
	err := HandleWith(context.Background(), []string{"daily"}, runner, RunDependencies{
		Loader: loader,
		Stat:   os.Stat,
		System: fakeExec,
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
	if len(fakeExec.runCalls) == 0 {
		t.Fatalf("expected windows executor calls")
	}
	initCall := strings.Join(fakeExec.runCalls[0], " ")
	if !strings.Contains(initCall, "restic.exe") {
		t.Fatalf("expected windows init via restic.exe, got %v", fakeExec.runCalls[0])
	}
	if !strings.Contains(initCall, "init --repo") {
		t.Fatalf("expected init command in windows call, got %v", fakeExec.runCalls[0])
	}
	if !strings.Contains(initCall, `C:\\missing\\repo`) && !strings.Contains(initCall, `C:\missing\repo`) {
		t.Fatalf("expected init call to target windows repository, got %v", fakeExec.runCalls[0])
	}
	if !strings.Contains(initCall, "--password-file") {
		t.Fatalf("expected password-file on windows init call, got %v", fakeExec.runCalls[0])
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

func TestHandleStopsAfterFirstProfileError(t *testing.T) {
	t.Setenv("RESTIC_PASSWORD", "test-password")

	rulesDir := withTempRules(t, "daily", []string{"wsl", "windows"}, []string{"wsl", "windows"})
	wslRepo := withTempRepository(t)
	runner := &fakeRunner{}
	fakeExec := &fakeSystem{
		runCapture: map[string]string{},
			runErr: map[string]error{
				"restic.exe --password-file C:\\rules\\backup-password.txt --repo C:\\repo\\windows backup --tag cadence=daily --tag profile=windows --files-from C:\\rules\\includes.daily.txt --exclude-file C:\\rules\\excludes.txt": fmt.Errorf("windows failed"),
		},
	}
	loader := fakeLoader{cfg: config.File{
		ResticVersion: "0.18.1",
		Profiles: map[string]config.Profile{
			"windows": {Repository: `C:\repo\windows`, UseFSSnapshot: false},
			"wsl":     {Repository: wslRepo, UseFSSnapshot: false},
		},
	}}
	loader.cfgPathSetForTest(rulesDir)
	fakeExec.runCapture["restic version"] = "restic 0.18.1 compiled with go"
	fakeExec.runCapture["pwsh.exe -NoProfile -Command restic version"] = "restic 0.18.1 compiled with go"
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-001.txt")] = `C:\rules\includes.daily.txt`
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-002.txt")] = `C:\rules\excludes.txt`
	fakeExec.runCapture["wslpath -w "+filepath.Join(rulesDir, "excludes.txt")] = `C:\rules\excludes.txt`

	originalCreateTemp := osCreateTemp
	createTempIndex := 0
	osCreateTemp = func(_ string, pattern string) (*os.File, error) {
		createTempIndex++
		name := fmt.Sprintf("wsl-backup-orchestrator-rule-%03d.txt", createTempIndex)
		if strings.Contains(pattern, "password") {
			name = "wsl-backup-orchestrator-password-001.txt"
		}
		path := filepath.Join(os.TempDir(), name)
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
	t.Cleanup(func() {
		osCreateTemp = originalCreateTemp
		_ = os.Remove(filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-001.txt"))
		_ = os.Remove(filepath.Join(os.TempDir(), "wsl-backup-orchestrator-rule-002.txt"))
		_ = os.Remove(filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-001.txt"))
	})
	fakeExec.runCapture["wslpath -w "+filepath.Join(os.TempDir(), "wsl-backup-orchestrator-password-001.txt")] = `C:\rules\backup-password.txt`

	err := HandleWith(context.Background(), []string{"daily"}, runner, RunDependencies{Loader: loader, Stat: os.Stat, System: fakeExec})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "profile windows") {
		t.Fatalf("expected windows profile error, got %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("expected wsl profile not to run after windows error, got %d calls", len(runner.calls))
	}
}

func TestHandlePrintsProfilePrefix(t *testing.T) {
	t.Setenv("RESTIC_PASSWORD", "test-password")

	rulesDir := withTempRules(t, "weekly", []string{"wsl"}, []string{"wsl"})
	wslRepo := withTempRepository(t)
	runner := &fakeRunner{}
	fakeExec := &fakeSystem{runCapture: map[string]string{}}
	loader := fakeLoader{cfg: config.File{
		ResticVersion: "0.18.1",
		Profiles: map[string]config.Profile{
			"wsl": {Repository: wslRepo},
		},
	}}
	loader.cfgPathSetForTest(rulesDir)
	fakeExec.runCapture["restic version"] = "restic 0.18.1 compiled with go"

	var output strings.Builder
	err := HandleWith(context.Background(), []string{"weekly"}, runner, RunDependencies{
		Loader: loader,
		Stat:   os.Stat,
		System: fakeExec,
		Output: &output,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(output.String(), "[wsl]") {
		t.Fatalf("expected profile prefix in output, got %q", output.String())
	}
}

func (l *fakeLoader) cfgPathSetForTest(dir string) {
	l.cfg = config.FileWithPathForTest(l.cfg, filepath.Join(dir, "config.yaml"))
}

func TestBuildRunArgsErrorsOnUnexpectedExcludeStatFailure(t *testing.T) {
	_, err := buildRunArgs("/cfg", "wsl", config.Profile{Repository: "/repo"}, "daily", nil, func(path string) (os.FileInfo, error) {
		if strings.Contains(path, "includes.") {
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

func TestBuildRunArgsWeeklyIncludesDailyAndWeeklyRuleFiles(t *testing.T) {
	rulesDir := withTempRules(t, "weekly", []string{"wsl"}, []string{"wsl"})

	args, err := buildRunArgs(rulesDir, "wsl", config.Profile{Repository: "/repo"}, "weekly", nil, os.Stat)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--files-from "+filepath.Join(rulesDir, "includes.daily.txt")) {
		t.Fatalf("expected daily include rules in args: %v", args)
	}
	if !strings.Contains(joined, "--files-from "+filepath.Join(rulesDir, "includes.weekly.txt")) {
		t.Fatalf("expected weekly include rules in args: %v", args)
	}
	if !strings.Contains(joined, "--exclude-file "+filepath.Join(rulesDir, "excludes.txt")) {
		t.Fatalf("expected profile exclude rules in args: %v", args)
	}
}

func TestBuildRunArgsMonthlyIncludesDailyWeeklyAndMonthlyRuleFiles(t *testing.T) {
	rulesDir := withTempRules(t, "monthly", []string{"wsl"}, []string{"wsl"})

	args, err := buildRunArgs(rulesDir, "wsl", config.Profile{Repository: "/repo"}, "monthly", nil, os.Stat)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	joined := strings.Join(args, " ")
	for _, cadence := range []string{"daily", "weekly", "monthly"} {
		if !strings.Contains(joined, "--files-from "+filepath.Join(rulesDir, "includes."+cadence+".txt")) {
			t.Fatalf("expected %s include rules in args: %v", cadence, args)
		}
	}
	if !strings.Contains(joined, "--exclude-file "+filepath.Join(rulesDir, "excludes.txt")) {
		t.Fatalf("expected profile exclude rules in args: %v", args)
	}
}

func TestValidateIncludeRuleOverlapFailsOnCrossProfileOverlap(t *testing.T) {
	dir := t.TempDir()
	if err := validateIncludeRuleOverlap(dir, "daily", map[string]config.Profile{
		"wsl":     {Repository: "/repo/wsl"},
		"windows": {Repository: `C:\repo`},
	}, os.ReadFile); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateIncludeRuleOverlapAllowsDistinctPaths(t *testing.T) {
	if err := validateIncludeRuleOverlap(t.TempDir(), "daily", map[string]config.Profile{
		"wsl":     {Repository: "/repo/wsl"},
		"windows": {Repository: `C:\repo`},
	}, os.ReadFile); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateIncludeRuleOverlapAllowsExcludeOverlap(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "excludes.txt"), []byte("/mnt/c/Users/mike\nC:\\Users\\mike\\docs\n"), 0o644); err != nil {
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
	if err := validateIncludeRuleOverlap(dir, "daily", map[string]config.Profile{
		"wsl":     {Repository: "/repo/wsl"},
		"windows": {Repository: `C:\repo`},
	}, os.ReadFile); err != nil {
		t.Fatalf("expected nil error, got %v", err)
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

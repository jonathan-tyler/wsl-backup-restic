package integration

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/commands/run"
	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/config"
)

type snapshotLoader struct {
	cfg config.File
}

func (l snapshotLoader) Load() (config.File, error) {
	return l.cfg, nil
}

type snapshotSystem struct {
	runCapture map[string]string
}

func (s *snapshotSystem) Run(_ context.Context, _ string, _ ...string) error {
	return nil
}

func (s *snapshotSystem) RunWithEnv(_ context.Context, _ map[string]string, _ string, _ ...string) error {
	return nil
}

func (s *snapshotSystem) RunCapture(_ context.Context, name string, args ...string) (string, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	if out, ok := s.runCapture[key]; ok {
		return out, nil
	}
	return "", nil
}

type snapshotRunner struct{}

func (r *snapshotRunner) Run(_ context.Context, args ...string) error {
	if len(args) > 0 && args[0] == "snapshots" {
		return nil
	}

	repo, cadence, includeFiles, excludeFiles, err := parseBackupArgs(args)
	if err != nil {
		return err
	}

	included, err := resolveRuleFilePaths(includeFiles)
	if err != nil {
		return err
	}
	excludeRules, err := resolveRulePatterns(excludeFiles)
	if err != nil {
		return err
	}

	selected := make([]string, 0, len(included))
	for path := range included {
		if matchesAnyExcludeRule(path, excludeRules) {
			continue
		}
		selected = append(selected, path)
	}
	sort.Strings(selected)

	if err := os.MkdirAll(filepath.Join(repo, "snapshots"), 0o755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	content := strings.Join(selected, "\n")
	if len(selected) > 0 {
		content += "\n"
	}

	if err := os.WriteFile(filepath.Join(repo, "snapshots", cadence+".txt"), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}

	return nil
}

func parseBackupArgs(args []string) (string, string, []string, []string, error) {
	repo := ""
	cadence := ""
	includeFiles := make([]string, 0)
	excludeFiles := make([]string, 0)

	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--repo":
			if index+1 >= len(args) {
				return "", "", nil, nil, fmt.Errorf("missing repo value")
			}
			repo = args[index+1]
			index++
		case "--tag":
			if index+1 >= len(args) {
				return "", "", nil, nil, fmt.Errorf("missing tag value")
			}
			value := args[index+1]
			if strings.HasPrefix(value, "cadence=") {
				cadence = strings.TrimPrefix(value, "cadence=")
			}
			index++
		case "--files-from":
			if index+1 >= len(args) {
				return "", "", nil, nil, fmt.Errorf("missing files-from value")
			}
			includeFiles = append(includeFiles, args[index+1])
			index++
		case "--exclude-file":
			if index+1 >= len(args) {
				return "", "", nil, nil, fmt.Errorf("missing exclude-file value")
			}
			excludeFiles = append(excludeFiles, args[index+1])
			index++
		}
	}

	if strings.TrimSpace(repo) == "" {
		return "", "", nil, nil, fmt.Errorf("missing repo arg")
	}
	if strings.TrimSpace(cadence) == "" {
		return "", "", nil, nil, fmt.Errorf("missing cadence tag")
	}

	return repo, cadence, includeFiles, excludeFiles, nil
}

func resolveRuleFilePaths(ruleFiles []string) (map[string]struct{}, error) {
	items := map[string]struct{}{}
	for _, ruleFile := range ruleFiles {
		content, err := os.ReadFile(ruleFile)
		if err != nil {
			return nil, fmt.Errorf("read rules file %s: %w", ruleFile, err)
		}

		for _, line := range strings.Split(string(content), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			items[trimmed] = struct{}{}
		}
	}

	return items, nil
}

func resolveRulePatterns(ruleFiles []string) ([]string, error) {
	items := make([]string, 0)
	for _, ruleFile := range ruleFiles {
		content, err := os.ReadFile(ruleFile)
		if err != nil {
			return nil, fmt.Errorf("read rules file %s: %w", ruleFile, err)
		}

		for _, line := range strings.Split(string(content), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			items = append(items, trimmed)
		}
	}

	return items, nil
}

func matchesAnyExcludeRule(filePath string, patterns []string) bool {
	normalizedPath := filepath.ToSlash(filePath)
	baseName := filepath.Base(normalizedPath)

	for _, patternText := range patterns {
		normalizedPattern := filepath.ToSlash(strings.TrimSpace(patternText))
		if normalizedPattern == "" {
			continue
		}

		if strings.ContainsAny(normalizedPattern, "*?[") {
			if matched, err := path.Match(normalizedPattern, normalizedPath); err == nil && matched {
				return true
			}
			if matched, err := path.Match(normalizedPattern, baseName); err == nil && matched {
				return true
			}
			continue
		}

		if normalizedPath == normalizedPattern {
			return true
		}
	}

	return false
}

func writeIncludeRuleSet(t *testing.T, rulesDir string, cadence string, include []string) {
	t.Helper()

	includePath := filepath.Join(rulesDir, fmt.Sprintf("includes.%s.txt", cadence))

	includeContent := strings.Join(include, "\n")
	if len(include) > 0 {
		includeContent += "\n"
	}

	if err := os.WriteFile(includePath, []byte(includeContent), 0o644); err != nil {
		t.Fatalf("write include rules: %v", err)
	}

}

func writeExcludeRules(t *testing.T, rulesDir string, exclude []string) {
	t.Helper()

	excludePath := filepath.Join(rulesDir, "excludes.txt")
	excludeContent := strings.Join(exclude, "\n")
	if len(exclude) > 0 {
		excludeContent += "\n"
	}

	if err := os.WriteFile(excludePath, []byte(excludeContent), 0o644); err != nil {
		t.Fatalf("write exclude rules: %v", err)
	}
}

func readSnapshotEntries(t *testing.T, repository string, cadence string) []string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(repository, "snapshots", cadence+".txt"))
	if err != nil {
		t.Fatalf("read snapshot content: %v", err)
	}

	entries := make([]string, 0)
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			entries = append(entries, trimmed)
		}
	}
	sort.Strings(entries)

	return entries
}

func TestRunCadenceInheritanceSnapshotFileCounts(t *testing.T) {
	t.Setenv("RESTIC_PASSWORD", "test-password")

	sourceDir := t.TempDir()
	repositoryDir := t.TempDir()
	rulesDir := t.TempDir()

	dailyFile1 := filepath.Join(sourceDir, "daily1.txt")
	dailyFile2 := filepath.Join(sourceDir, "daily2.txt")
	weeklyFile3 := filepath.Join(sourceDir, "weekly3.txt")
	weeklyFile4 := filepath.Join(sourceDir, "weekly4.txt")
	monthlyFile5 := filepath.Join(sourceDir, "monthly5.txt")
	monthlyFile6 := filepath.Join(sourceDir, "monthly6.txt")

	for _, item := range []string{dailyFile1, dailyFile2, weeklyFile3, weeklyFile4, monthlyFile5, monthlyFile6} {
		if err := os.WriteFile(item, []byte("content\n"), 0o644); err != nil {
			t.Fatalf("write source file: %v", err)
		}
	}

	writeIncludeRuleSet(t, rulesDir, "daily", []string{dailyFile1, dailyFile2})
	writeIncludeRuleSet(t, rulesDir, "weekly", []string{weeklyFile3, weeklyFile4})
	writeIncludeRuleSet(t, rulesDir, "monthly", []string{monthlyFile5, monthlyFile6})
	writeExcludeRules(t, rulesDir, []string{"*1*", "*3*", "*5*"})

	if err := os.WriteFile(filepath.Join(repositoryDir, "config"), []byte("repo-config"), 0o644); err != nil {
		t.Fatalf("write repository config: %v", err)
	}

	loader := snapshotLoader{cfg: config.FileWithPathForTest(config.File{
		ResticVersion: "0.18.1",
		Profiles: map[string]config.Profile{
			"wsl": {Repository: repositoryDir},
		},
	}, filepath.Join(rulesDir, "config.yaml"))}

	system := &snapshotSystem{runCapture: map[string]string{
		"restic version": "restic 0.18.1 compiled with go",
	}}

	runner := &snapshotRunner{}

	tests := []struct {
		cadence string
		want    int
		files   []string
	}{
		{cadence: "daily", want: 1, files: []string{dailyFile2}},
		{cadence: "weekly", want: 2, files: []string{dailyFile2, weeklyFile4}},
		{cadence: "monthly", want: 3, files: []string{dailyFile2, weeklyFile4, monthlyFile6}},
	}

	for _, tt := range tests {
		t.Run(tt.cadence, func(t *testing.T) {
			err := run.HandleWith(context.Background(), []string{tt.cadence}, runner, run.RunDependencies{
				Loader: loader,
				Stat:   os.Stat,
				System: system,
				Output: io.Discard,
			})
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}

			got := readSnapshotEntries(t, repositoryDir, tt.cadence)
			if len(got) != tt.want {
				t.Fatalf("unexpected snapshot file count for %s: got %d want %d (files: %v)", tt.cadence, len(got), tt.want, got)
			}

			wantFiles := append([]string{}, tt.files...)
			sort.Strings(wantFiles)
			if !reflect.DeepEqual(got, wantFiles) {
				t.Fatalf("unexpected snapshot files for %s: got %v want %v", tt.cadence, got, wantFiles)
			}
		})
	}
}

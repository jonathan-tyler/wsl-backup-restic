package run

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
)

type readFileFunc func(string) ([]byte, error)

func validateIncludeRuleOverlap(configDir string, cadence string, profiles map[string]config.Profile, readFile readFileFunc) error {
	if err := validateRuleKindOverlap(configDir, cadence, profiles, readFile, "include"); err != nil {
		return err
	}

	if err := validateRuleKindOverlap(configDir, cadence, profiles, readFile, "exclude"); err != nil {
		return err
	}

	return nil
}

func validateRuleKindOverlap(configDir string, cadence string, profiles map[string]config.Profile, readFile readFileFunc, kind string) error {
	items := make([]includeEntry, 0)
	for profileName := range profiles {
		rulesPath := rulesFilePath(configDir, profileName, cadence, kind)
		content, err := readFile(rulesPath)
		if err != nil {
			if kind == "exclude" && os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("profile %s read %s rules failed: %w", profileName, kind, err)
		}

		rules := parseRuleLines(string(content))
		for _, rule := range rules {
			items = append(items, includeEntry{
				Profile: profileName,
				Raw:     rule,
				Norm:    normalizePath(rule),
			})
		}
	}

	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].Profile == items[j].Profile {
				continue
			}
			if isPathOverlap(items[i].Norm, items[j].Norm) {
				return fmt.Errorf(
					"%s rules overlap for cadence %s: profile %s path %q overlaps profile %s path %q",
					kind,
					cadence,
					items[i].Profile,
					items[i].Raw,
					items[j].Profile,
					items[j].Raw,
				)
			}
		}
	}

	return nil
}

func rulesFilePath(configDir string, profile string, cadence string, kind string) string {
	if kind == "exclude" {
		return config.ExcludeRulesPath(configDir, profile, cadence)
	}
	return config.IncludeRulesPath(configDir, profile, cadence)
}

type includeEntry struct {
	Profile string
	Raw     string
	Norm    string
}

func parseRuleLines(content string) []string {
	lines := strings.Split(content, "\n")
	rules := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		rules = append(rules, trimmed)
	}
	sort.Strings(rules)
	return rules
}

func normalizePath(value string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	normalized = normalizeWSLMountPath(normalized)
	normalized = strings.TrimSuffix(normalized, "/")
	normalized = filepath.Clean(normalized)
	if len(normalized) >= 2 && normalized[1] == ':' {
		normalized = strings.ToLower(normalized)
	}
	return normalized
}

func normalizeWSLMountPath(value string) string {
	if !strings.HasPrefix(value, "/mnt/") || len(value) < len("/mnt/c") {
		return value
	}

	drive := value[len("/mnt/")]
	if !isASCIILetter(drive) {
		return value
	}

	if len(value) == len("/mnt/c") {
		return strings.ToLower(string(drive)) + ":/"
	}

	if value[len("/mnt/c")] != '/' {
		return value
	}

	return strings.ToLower(string(drive)) + ":" + value[len("/mnt/c"):]
}

func isASCIILetter(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}

func isPathOverlap(a string, b string) bool {
	if a == b {
		return true
	}
	if strings.HasPrefix(a, b+"/") {
		return true
	}
	if strings.HasPrefix(b, a+"/") {
		return true
	}
	return false
}

package run

import (
	"fmt"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
)

func inheritedCadences(cadence string) ([]string, error) {
	switch cadence {
	case "daily":
		return []string{"daily"}, nil
	case "weekly":
		return []string{"daily", "weekly"}, nil
	case "monthly":
		return []string{"daily", "weekly", "monthly"}, nil
	default:
		return nil, fmt.Errorf("unsupported cadence %q", cadence)
	}
}

func includeRulePaths(configDir string, profileName string, cadence string) ([]string, error) {
	items, err := inheritedCadences(cadence)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(items))
	for _, item := range items {
		paths = append(paths, config.IncludeRulesPath(configDir, profileName, item))
	}

	return paths, nil
}

func excludeRulePaths(configDir string, profileName string, cadence string) ([]string, error) {
	items, err := inheritedCadences(cadence)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(items))
	for _, item := range items {
		paths = append(paths, config.ExcludeRulesPath(configDir, profileName, item))
	}

	return paths, nil
}

package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/prompt"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/restic"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/system"
)

func runPreflight(
	ctx context.Context,
	cfg config.File,
	cadence string,
	profiles map[string]config.Profile,
	stat fileStatFunc,
	runner restic.Executor,
	exec system.Executor,
	confirm prompt.ConfirmFunc,
) error {
	if err := validateRuleFilesExist(cfg.Dir(), cadence, profiles, stat); err != nil {
		return err
	}

	if hasProfiles(profiles) {
		if err := restic.CheckPasswordConfigured(); err != nil {
			return err
		}
	}

	for profileName, profile := range profiles {
		if err := ensureRepositoryReady(ctx, profileName, profile.Repository, stat, runner, exec, confirm); err != nil {
			return err
		}
	}

	return nil
}

func validateRuleFilesExist(configDir string, cadence string, profiles map[string]config.Profile, stat fileStatFunc) error {
	for profileName := range profiles {
		includePath := config.IncludeRulesPath(configDir, profileName, cadence)
		if _, err := stat(includePath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("profile %s missing include rules file: %s", profileName, includePath)
			}
			return fmt.Errorf("profile %s include rules check failed: %w", profileName, err)
		}

		excludePath := config.ExcludeRulesPath(configDir, profileName, cadence)
		if _, err := stat(excludePath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("profile %s missing exclude rules file: %s", profileName, excludePath)
			}
			return fmt.Errorf("profile %s exclude rules check failed: %w", profileName, err)
		}
	}

	return nil
}

func hasProfiles(profiles map[string]config.Profile) bool {
	return len(profiles) > 0
}

func ensureRepositoryReady(
	ctx context.Context,
	profileName string,
	repository string,
	stat fileStatFunc,
	runner restic.Executor,
	exec system.Executor,
	confirm prompt.ConfirmFunc,
) error {
	if strings.TrimSpace(repository) == "" {
		return fmt.Errorf("profile %s has empty repository", profileName)
	}

	exists, checked, err := repositoryConfigExists(profileName, repository, stat)
	if err != nil {
		return fmt.Errorf("profile %s repository check failed: %w", profileName, err)
	}
	if !checked || exists {
		return nil
	}

	create := false
	if confirm != nil {
		promptText := fmt.Sprintf("Repository for profile %s is missing at %s. Create it now?", profileName, repository)
		answer, confirmErr := confirm(promptText)
		if confirmErr != nil {
			return fmt.Errorf("profile %s repository creation prompt failed: %w", profileName, confirmErr)
		}
		create = answer
	}

	if !create {
		return fmt.Errorf("profile %s repository missing: %s", profileName, repository)
	}

	if strings.EqualFold(profileName, "windows") {
		if err := exec.Run(ctx, "restic.exe", "init", "--repo", repository); err != nil {
			return fmt.Errorf("profile %s repository init failed: %w", profileName, err)
		}
		return nil
	}

	if err := runner.Run(ctx, "init", "--repo", repository); err != nil {
		return fmt.Errorf("profile %s repository init failed: %w", profileName, err)
	}
	return nil
}

func repositoryConfigExists(profileName string, repository string, stat fileStatFunc) (bool, bool, error) {
	configPath := filepath.Join(repository, "config")
	if strings.EqualFold(profileName, "windows") {
		if converted, ok := windowsPathToWSL(repository); ok {
			configPath = filepath.Join(converted, "config")
		}
	}

	_, err := stat(configPath)
	if err == nil {
		return true, true, nil
	}
	if os.IsNotExist(err) {
		return false, true, nil
	}

	return false, true, err
}

func windowsPathToWSL(path string) (string, bool) {
	if len(path) < 3 || path[1] != ':' {
		return "", false
	}

	separator := path[2]
	if separator != '\\' && separator != '/' {
		return "", false
	}

	drive := strings.ToLower(path[:1])
	rest := strings.ReplaceAll(path[2:], "\\", "/")
	return "/mnt/" + drive + rest, true
}

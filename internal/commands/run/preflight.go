package run

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/config"
	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/prompt"
	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/restic"
	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/system"
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
	passwordPrompt prompt.PasswordFunc,
) error {
	if err := validateRuleFilesExist(cfg.Dir(), cadence, profiles, stat); err != nil {
		return err
	}

	if hasProfiles(profiles) {
		if err := validateRepositoryUniqueness(profiles, cadence); err != nil {
			return err
		}
	}

	for _, profileName := range sortedProfileNames(profiles) {
		if err := ensureRepositoryReady(ctx, profileName, profiles[profileName], cadence, stat, runner, exec, confirm, passwordPrompt); err != nil {
			return err
		}
	}

	if hasProfiles(profiles) {
		if err := ensureResticPassword(profiles, cadence, stat, passwordPrompt, false); err != nil {
			return err
		}
	}

	return nil
}

func validateRepositoryUniqueness(profiles map[string]config.Profile, cadence string) error {
	seen := map[string]string{}
	raw := map[string]string{}

	for profileName, profile := range profiles {
		repository, err := profile.RepositoryFor(cadence)
		if err != nil {
			return fmt.Errorf("profile %s repository lookup failed: %w", profileName, err)
		}
		normalized := normalizePath(repository)
		if priorProfile, exists := seen[normalized]; exists {
			return fmt.Errorf(
				"profiles %s and %s target the same %s repository after normalization: %q and %q",
				priorProfile,
				profileName,
				cadence,
				raw[normalized],
				repository,
			)
		}

		seen[normalized] = profileName
		raw[normalized] = repository
	}

	return nil
}

func validateRuleFilesExist(configDir string, cadence string, profiles map[string]config.Profile, stat fileStatFunc) error {
	for profileName := range profiles {
		includePaths, err := includeRulePaths(configDir, profileName, cadence)
		if err != nil {
			return err
		}
		for _, includePath := range includePaths {
			if _, err := stat(includePath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("profile %s missing include rules file: %s", profileName, includePath)
				}
				return fmt.Errorf("profile %s include rules check failed: %w", profileName, err)
			}
		}

		excludePaths, err := excludeRulePaths(configDir, profileName, cadence)
		if err != nil {
			return err
		}
		for _, excludePath := range excludePaths {
			if _, err := stat(excludePath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("profile %s missing exclude rules file: %s", profileName, excludePath)
				}
				return fmt.Errorf("profile %s exclude rules check failed: %w", profileName, err)
			}
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
	profile config.Profile,
	cadence string,
	stat fileStatFunc,
	runner restic.Executor,
	exec system.Executor,
	confirm prompt.ConfirmFunc,
	passwordPrompt prompt.PasswordFunc,
) error {
	repository, err := profile.RepositoryFor(cadence)
	if err != nil {
		return fmt.Errorf("profile %s repository lookup failed: %w", profileName, err)
	}
	if strings.TrimSpace(repository) == "" {
		return fmt.Errorf("profile %s has empty %s repository", profileName, cadence)
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
		promptText := fmt.Sprintf("Repository for profile %s cadence %s is missing at %s. Create it now?", profileName, cadence, repository)
		answer, confirmErr := confirm(promptText)
		if confirmErr != nil {
			return fmt.Errorf("profile %s repository creation prompt failed: %w", profileName, confirmErr)
		}
		create = answer
	}

	if !create {
		return fmt.Errorf("profile %s %s repository missing: %s", profileName, cadence, repository)
	}

	if err := ensureResticPassword(map[string]config.Profile{profileName: profile}, cadence, stat, passwordPrompt, true); err != nil {
		return err
	}

	if strings.EqualFold(profileName, "windows") {
		if err := runWindowsResticCommand(ctx, []string{"init", "--repo", repository}, false, exec); err != nil {
			return fmt.Errorf("profile %s repository init failed: %w", profileName, err)
		}
		return nil
	}

	if err := runner.Run(ctx, "init", "--repo", repository); err != nil {
		return fmt.Errorf("profile %s repository init failed: %w", profileName, err)
	}
	return nil
}

func ensureResticPassword(
	profiles map[string]config.Profile,
	cadence string,
	stat fileStatFunc,
	passwordPrompt prompt.PasswordFunc,
	forcePrompt bool,
) error {
	if err := restic.CheckPasswordConfigured(); err == nil {
		return nil
	} else if !errors.Is(err, restic.ErrPasswordNotConfigured) {
		return err
	} else if !forcePrompt {
		hasExistingRepository, existsErr := anyRepositoryExists(profiles, cadence, stat)
		if existsErr != nil {
			return existsErr
		}
		if !hasExistingRepository {
			return err
		}
	}

	if passwordPrompt == nil {
		return restic.ErrPasswordNotConfigured
	}

	password, promptErr := passwordPrompt("Restic password")
	if promptErr != nil {
		return fmt.Errorf("restic password prompt failed: %w", promptErr)
	}

	if err := os.Setenv("RESTIC_PASSWORD", strings.TrimSpace(password)); err != nil {
		return fmt.Errorf("set RESTIC_PASSWORD: %w", err)
	}

	return nil
}

func anyRepositoryExists(profiles map[string]config.Profile, cadence string, stat fileStatFunc) (bool, error) {
	for profileName, profile := range profiles {
		repository, err := profile.RepositoryFor(cadence)
		if err != nil {
			return false, fmt.Errorf("profile %s repository lookup failed: %w", profileName, err)
		}
		exists, checked, err := repositoryConfigExists(profileName, repository, stat)
		if err != nil {
			return false, fmt.Errorf("profile %s repository check failed: %w", profileName, err)
		}
		if checked && exists {
			return true, nil
		}
	}

	return false, nil
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

package run

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/apperr"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/prompt"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/restic"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/resticversion"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/system"
)

type ConfigLoader interface {
	Load() (config.File, error)
}

type fileStatFunc func(string) (os.FileInfo, error)

type RunDependencies struct {
	Loader  ConfigLoader
	Stat    fileStatFunc
	System  system.Executor
	Confirm prompt.ConfirmFunc
	Output  io.Writer
}

func Handle(ctx context.Context, args []string, runner restic.Executor) error {
	deps := RunDependencies{
		Loader:  config.NewLoader(),
		Stat:    os.Stat,
		System:  system.NewOSExecutor(os.Stdout, os.Stderr),
		Confirm: prompt.NewYesNoConfirm(os.Stdin, os.Stdout),
		Output:  os.Stdout,
	}

	return HandleWith(ctx, args, runner, deps)
}

func HandleWith(ctx context.Context, args []string, runner restic.Executor, deps RunDependencies) error {
	if len(args) == 0 {
		return apperr.UsageError{Message: "missing cadence: expected one of daily, weekly, monthly"}
	}

	cadence := args[0]
	if !isValidCadence(cadence) {
		return apperr.UsageError{Message: fmt.Sprintf("invalid cadence %q: expected one of daily, weekly, monthly", cadence)}
	}

	if deps.Loader == nil {
		deps.Loader = config.NewLoader()
	}
	if deps.Stat == nil {
		deps.Stat = os.Stat
	}
	if deps.System == nil {
		deps.System = system.NewOSExecutor(os.Stdout, os.Stderr)
	}
	if deps.Confirm == nil {
		deps.Confirm = func(string) (bool, error) { return false, nil }
	}
	if deps.Output == nil {
		deps.Output = os.Stdout
	}

	cfg, err := deps.Loader.Load()
	if err != nil {
		return err
	}

	if err := resticversion.CheckCompatible(ctx, cfg, deps.System); err != nil {
		return err
	}

	if err := validateIncludeRuleOverlap(cfg.Dir(), cadence, cfg.Profiles, os.ReadFile); err != nil {
		return err
	}

	if err := runPreflight(ctx, cfg, cadence, cfg.Profiles, deps.Stat, runner, deps.System, deps.Confirm); err != nil {
		return err
	}

	profileNames := sortedProfileNames(cfg.Profiles)
	if len(profileNames) == 0 {
		return fmt.Errorf("no profiles configured")
	}

	for _, profileName := range profileNames {
		profile := cfg.Profiles[profileName]
		fmt.Fprintf(deps.Output, "\n[%s]\n", profileName)
		resticArgs, err := buildRunArgs(cfg.Dir(), profileName, profile, cadence, args[1:], deps.Stat)
		if err != nil {
			return err
		}

		if err := executeProfileBackup(ctx, profileName, profile, resticArgs, runner, deps.System); err != nil {
			return fmt.Errorf("profile %s: %w", profileName, err)
		}
	}

	return nil
}

func isValidCadence(value string) bool {
	switch value {
	case "daily", "weekly", "monthly":
		return true
	default:
		return false
	}
}

func buildRunArgs(configDir string, profileName string, profile config.Profile, cadence string, extraArgs []string, stat fileStatFunc) ([]string, error) {
	resticArgs := []string{"--repo", profile.Repository, "backup", "--tag", "cadence=" + cadence, "--tag", "profile=" + profileName}

	if strings.EqualFold(profileName, "windows") && profile.UseFSSnapshot {
		resticArgs = append(resticArgs, "--use-fs-snapshot")
	}

	includePaths, err := includeRulePaths(configDir, profileName, cadence)
	if err != nil {
		return nil, err
	}
	for _, includePath := range includePaths {
		if _, err := stat(includePath); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("profile %s missing include rules file: %s", profileName, includePath)
			}
			return nil, fmt.Errorf("profile %s include rules check failed: %w", profileName, err)
		}
		resticArgs = append(resticArgs, "--files-from", includePath)
	}

	excludePaths, err := excludeRulePaths(configDir, profileName, cadence)
	if err != nil {
		return nil, err
	}
	for _, excludePath := range excludePaths {
		if _, err := stat(excludePath); err == nil {
			resticArgs = append(resticArgs, "--exclude-file", excludePath)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("profile %s exclude rules check failed: %w", profileName, err)
		}
	}

	resticArgs = append(resticArgs, extraArgs...)
	return resticArgs, nil
}

func sortedProfileNames(profiles map[string]config.Profile) []string {
	items := make([]string, 0, len(profiles))
	for name := range profiles {
		items = append(items, name)
	}
	sort.Strings(items)
	return items
}

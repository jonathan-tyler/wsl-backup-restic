package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/apperr"
	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/config"
	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/prompt"
	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/restic"
	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/resticversion"
	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/system"
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

type profileExecutionPlan struct {
	name string
	args []string
	err  error
}

type profileSnapshotPlan struct {
	name string
	args []string
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

	if err := runPreflight(ctx, cfg, cadence, cfg.Profiles, deps.Stat, runner, deps.System, deps.Confirm); err != nil {
		return err
	}

	profileNames := sortedProfileNames(cfg.Profiles)
	if len(profileNames) == 0 {
		return fmt.Errorf("no profiles configured")
	}

	plans := make([]profileExecutionPlan, 0, len(profileNames))
	for _, profileName := range profileNames {
		profile := cfg.Profiles[profileName]
		fmt.Fprintf(deps.Output, "\n[%s]\n", profileName)
		resticArgs, err := buildRunArgs(cfg.Dir(), profileName, profile, cadence, args[1:], deps.Stat)
		if err != nil {
			return err
		}
		plans = append(plans, profileExecutionPlan{name: profileName, args: resticArgs})
	}

	results := make([]profileExecutionPlan, len(plans))
	var wg sync.WaitGroup
	for index, plan := range plans {
		wg.Add(1)
		go func(index int, plan profileExecutionPlan) {
			defer wg.Done()
			profile := cfg.Profiles[plan.name]
			results[index] = plan
			results[index].err = executeProfileBackup(ctx, plan.name, profile, plan.args, runner, deps.System)
		}(index, plan)
	}
	wg.Wait()

	profileErrs := make([]error, 0)
	for _, result := range results {
		if result.err != nil {
			profileErrs = append(profileErrs, fmt.Errorf("profile %s: %w", result.name, result.err))
		}
	}
	if len(profileErrs) == 0 {
		snapshotPlans, err := buildProfileSnapshotPlans(cfg.Profiles)
		if err != nil {
			return err
		}
		for _, plan := range snapshotPlans {
			fmt.Fprintf(deps.Output, "\n[%s snapshots]\n", plan.name)
			if err := runner.Run(ctx, plan.args...); err != nil {
				return fmt.Errorf("profile %s snapshots: %w", plan.name, err)
			}
		}
		return nil
	}
	if len(profileErrs) == 1 {
		return profileErrs[0]
	}

	return errors.Join(profileErrs...)
}

func buildProfileSnapshotPlans(profiles map[string]config.Profile) ([]profileSnapshotPlan, error) {
	profileNames := sortedProfileNames(profiles)
	plans := make([]profileSnapshotPlan, 0, len(profileNames))
	for _, profileName := range profileNames {
		repository := profiles[profileName].Repository
		if strings.EqualFold(profileName, "windows") && looksLikeWindowsPath(repository) {
			converted, ok := windowsPathToWSL(repository)
			if !ok {
				return nil, fmt.Errorf("profile %s snapshots repository path %q is not a supported windows path", profileName, repository)
			}
			repository = converted
		}
		plans = append(plans, profileSnapshotPlan{
			name: profileName,
			args: []string{"snapshots", "--repo", repository},
		})
	}
	return plans, nil
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

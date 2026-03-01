package run

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/example/wsl-backup/internal/apperr"
	"github.com/example/wsl-backup/internal/config"
	"github.com/example/wsl-backup/internal/restic"
)

type ConfigLoader interface {
	Load() (config.File, error)
}

type fileStatFunc func(string) (os.FileInfo, error)

func Handle(ctx context.Context, args []string, runner restic.Executor) error {
	return HandleWith(ctx, args, runner, config.NewLoader(), os.Stat)
}

func HandleWith(ctx context.Context, args []string, runner restic.Executor, loader ConfigLoader, stat fileStatFunc) error {
	if len(args) == 0 {
		return apperr.UsageError{Message: "missing cadence: expected one of daily, weekly, monthly"}
	}

	cadence := args[0]
	if !isValidCadence(cadence) {
		return apperr.UsageError{Message: fmt.Sprintf("invalid cadence %q: expected one of daily, weekly, monthly", cadence)}
	}
	cfg, err := loader.Load()
	if err != nil {
		return err
	}

	profileNames := sortedProfileNames(cfg.Profiles)
	if len(profileNames) == 0 {
		return fmt.Errorf("no profiles configured")
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(profileNames))

	for _, profileName := range profileNames {
		profileName := profileName
		profile := cfg.Profiles[profileName]

		wg.Add(1)
		go func() {
			defer wg.Done()
			resticArgs, err := buildRunArgs(cfg.Dir(), profileName, profile, cadence, args[1:], stat)
			if err != nil {
				errCh <- err
				return
			}

			if err := runner.Run(ctx, resticArgs...); err != nil {
				errCh <- fmt.Errorf("profile %s: %w", profileName, err)
			}
		}()
	}

	wg.Wait()
	close(errCh)

	if len(errCh) > 0 {
		return <-errCh
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

	if profile.UseFSSnapshot {
		resticArgs = append(resticArgs, "--use-fs-snapshot")
	}

	includePath := config.IncludeRulesPath(configDir, profileName, cadence)
	if _, err := stat(includePath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("profile %s missing include rules file: %s", profileName, includePath)
		}
		return nil, fmt.Errorf("profile %s include rules check failed: %w", profileName, err)
	}
	resticArgs = append(resticArgs, "--files-from", includePath)

	excludePath := config.ExcludeRulesPath(configDir, profileName, cadence)
	if _, err := stat(excludePath); err == nil {
		resticArgs = append(resticArgs, "--exclude-file", excludePath)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %s exclude rules check failed: %w", profileName, err)
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

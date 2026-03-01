package resticversion

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/prompt"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/system"
)

type SetupStatus string

const (
	SetupMatched  SetupStatus = "matched"
	SetupInstalled SetupStatus = "installed"
	SetupUpgraded SetupStatus = "upgraded"
	SetupUpdated  SetupStatus = "updated"
	SetupFailed   SetupStatus = "failed"
	SetupSkipped  SetupStatus = "skipped"
)

type PlatformReport struct {
	Platform string
	Status   SetupStatus
	Message  string
}

type SetupReport struct {
	Items []PlatformReport
}

func (r *SetupReport) add(item PlatformReport) {
	r.Items = append(r.Items, item)
}

func CheckCompatible(ctx context.Context, cfg config.File, exec system.Executor) error {
	desiredVersion := strings.TrimSpace(cfg.ResticVersion)
	if desiredVersion == "" {
		return nil
	}

	if _, hasWSL := cfg.Profiles["wsl"]; hasWSL {
		if err := checkWSLCompatible(ctx, desiredVersion, exec); err != nil {
			return err
		}
	}

	if _, hasWindows := cfg.Profiles["windows"]; hasWindows {
		if err := checkWindowsCompatible(ctx, desiredVersion, exec); err != nil {
			return err
		}
	}

	return nil
}

func SyncInteractive(ctx context.Context, cfg config.File, exec system.Executor, confirm prompt.ConfirmFunc) error {
	_, err := SyncInteractiveWithReport(ctx, cfg, exec, confirm)
	return err
}

func SyncInteractiveWithReport(ctx context.Context, cfg config.File, exec system.Executor, confirm prompt.ConfirmFunc) (SetupReport, error) {
	report := SetupReport{}

	desiredVersion := strings.TrimSpace(cfg.ResticVersion)
	if desiredVersion == "" {
		report.add(PlatformReport{Platform: "global", Status: SetupSkipped, Message: "config has no restic_version; skipped"})
		return report, nil
	}

	if _, hasWSL := cfg.Profiles["wsl"]; hasWSL {
		item, err := syncWSLInteractive(ctx, desiredVersion, exec, confirm)
		report.add(item)
		if err != nil {
			return report, err
		}
	}

	if _, hasWindows := cfg.Profiles["windows"]; hasWindows {
		item, err := syncWindowsInteractive(ctx, desiredVersion, exec, confirm)
		report.add(item)
		if err != nil {
			return report, err
		}
	}

	return report, nil
}

var versionPattern = regexp.MustCompile(`\b(\d+\.\d+\.\d+)\b`)

func parseResticVersion(output string) (string, error) {
	match := versionPattern.FindStringSubmatch(output)
	if len(match) < 2 {
		return "", fmt.Errorf("could not find version in output %q", output)
	}
	return match[1], nil
}

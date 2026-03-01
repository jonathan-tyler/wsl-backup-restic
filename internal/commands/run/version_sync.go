package run

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/example/wsl-backup/internal/config"
	"github.com/example/wsl-backup/internal/prompt"
	"github.com/example/wsl-backup/internal/system"
)

func syncResticVersions(ctx context.Context, cfg config.File, exec system.Executor, confirm prompt.ConfirmFunc) error {
	desiredVersion := strings.TrimSpace(cfg.ResticVersion)
	if desiredVersion == "" {
		return nil
	}

	if _, hasWSL := cfg.Profiles["wsl"]; hasWSL {
		if err := syncWSLResticVersion(ctx, desiredVersion, exec, confirm); err != nil {
			return err
		}
	}

	if _, hasWindows := cfg.Profiles["windows"]; hasWindows {
		if err := syncWindowsResticVersion(ctx, desiredVersion, exec, confirm); err != nil {
			return err
		}
	}

	return nil
}

func syncWSLResticVersion(ctx context.Context, desiredVersion string, exec system.Executor, confirm prompt.ConfirmFunc) error {
	output, err := exec.RunCapture(ctx, "restic", "version")
	if err != nil {
		approved, confirmErr := confirm("WSL restic not found. Install via dnf now?")
		if confirmErr != nil {
			return confirmErr
		}
		if !approved {
			return fmt.Errorf("WSL restic is required")
		}
		return exec.Run(ctx, "sudo", "dnf", "install", "-y", "restic")
	}

	installedVersion, parseErr := parseResticVersion(output)
	if parseErr != nil {
		return fmt.Errorf("parse WSL restic version: %w", parseErr)
	}

	if installedVersion == desiredVersion {
		return nil
	}

	approved, confirmErr := confirm(fmt.Sprintf("WSL restic version is %s but config requires %s. Upgrade via dnf now?", installedVersion, desiredVersion))
	if confirmErr != nil {
		return confirmErr
	}
	if !approved {
		return fmt.Errorf("WSL restic version mismatch: installed=%s required=%s", installedVersion, desiredVersion)
	}

	return exec.Run(ctx, "sudo", "dnf", "upgrade", "-y", "restic")
}

func syncWindowsResticVersion(ctx context.Context, desiredVersion string, exec system.Executor, confirm prompt.ConfirmFunc) error {
	output, err := exec.RunCapture(ctx, "powershell.exe", "-NoProfile", "-Command", "restic version")
	if err != nil {
		approved, confirmErr := confirm("Windows restic not found. Install via scoop now?")
		if confirmErr != nil {
			return confirmErr
		}
		if !approved {
			return fmt.Errorf("windows restic is required")
		}
		return exec.Run(ctx, "powershell.exe", "-NoProfile", "-Command", "scoop install restic")
	}

	installedVersion, parseErr := parseResticVersion(output)
	if parseErr != nil {
		return fmt.Errorf("parse Windows restic version: %w", parseErr)
	}

	if installedVersion == desiredVersion {
		return nil
	}

	approved, confirmErr := confirm(fmt.Sprintf("Windows restic version is %s but config requires %s. Update via scoop now?", installedVersion, desiredVersion))
	if confirmErr != nil {
		return confirmErr
	}
	if !approved {
		return fmt.Errorf("windows restic version mismatch: installed=%s required=%s", installedVersion, desiredVersion)
	}

	return exec.Run(ctx, "powershell.exe", "-NoProfile", "-Command", "scoop update restic")
}

var versionPattern = regexp.MustCompile(`\b(\d+\.\d+\.\d+)\b`)

func parseResticVersion(output string) (string, error) {
	match := versionPattern.FindStringSubmatch(output)
	if len(match) < 2 {
		return "", fmt.Errorf("could not find version in output %q", output)
	}
	return match[1], nil
}

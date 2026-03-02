package resticversion

import (
	"context"
	"fmt"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/prompt"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/system"
)

func checkWindowsCompatible(ctx context.Context, desiredVersion string, exec system.Executor) error {
	output, err := exec.RunCapture(ctx, "pwsh.exe", "-NoProfile", "-Command", "restic version")
	if err != nil {
		return fmt.Errorf("windows restic is missing; run backup setup")
	}

	installedVersion, parseErr := parseResticVersion(output)
	if parseErr != nil {
		return fmt.Errorf("parse windows restic version: %w", parseErr)
	}

	if installedVersion != desiredVersion {
		return fmt.Errorf("windows restic version mismatch: installed=%s required=%s; run backup setup", installedVersion, desiredVersion)
	}

	return nil
}

func syncWindowsInteractive(ctx context.Context, desiredVersion string, exec system.Executor, confirm prompt.ConfirmFunc) (PlatformReport, error) {
	output, err := exec.RunCapture(ctx, "pwsh.exe", "-NoProfile", "-Command", "restic version")
	if err != nil {
		approved, confirmErr := confirm("Windows restic not found. Install via scoop now?")
		if confirmErr != nil {
			return PlatformReport{Platform: "windows", Status: SetupFailed, Message: "prompt failed"}, confirmErr
		}
		if !approved {
			failure := fmt.Errorf("windows restic is required")
			return PlatformReport{Platform: "windows", Status: SetupFailed, Message: failure.Error()}, failure
		}
		if runErr := exec.Run(ctx, "pwsh.exe", "-NoProfile", "-Command", "scoop install restic"); runErr != nil {
			return PlatformReport{Platform: "windows", Status: SetupFailed, Message: "install failed"}, runErr
		}
		return PlatformReport{Platform: "windows", Status: SetupInstalled, Message: "installed restic via scoop"}, nil
	}

	installedVersion, parseErr := parseResticVersion(output)
	if parseErr != nil {
		return PlatformReport{Platform: "windows", Status: SetupFailed, Message: "version parse failed"}, fmt.Errorf("parse windows restic version: %w", parseErr)
	}

	if installedVersion == desiredVersion {
		return PlatformReport{Platform: "windows", Status: SetupMatched, Message: fmt.Sprintf("version %s matches", desiredVersion)}, nil
	}

	approved, confirmErr := confirm(fmt.Sprintf("Windows restic version is %s but config requires %s. Update via scoop now?", installedVersion, desiredVersion))
	if confirmErr != nil {
		return PlatformReport{Platform: "windows", Status: SetupFailed, Message: "prompt failed"}, confirmErr
	}
	if !approved {
		failure := fmt.Errorf("windows restic version mismatch: installed=%s required=%s", installedVersion, desiredVersion)
		return PlatformReport{Platform: "windows", Status: SetupFailed, Message: failure.Error()}, failure
	}

	if runErr := exec.Run(ctx, "pwsh.exe", "-NoProfile", "-Command", "scoop update restic"); runErr != nil {
		return PlatformReport{Platform: "windows", Status: SetupFailed, Message: "update failed"}, runErr
	}
	return PlatformReport{Platform: "windows", Status: SetupUpdated, Message: fmt.Sprintf("updated from %s to %s", installedVersion, desiredVersion)}, nil
}

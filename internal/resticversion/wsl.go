package resticversion

import (
	"context"
	"fmt"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/prompt"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/system"
)

func checkWSLCompatible(ctx context.Context, desiredVersion string, exec system.Executor) error {
	output, err := exec.RunCapture(ctx, "restic", "version")
	if err != nil {
		return fmt.Errorf("wsl restic is missing; run backup setup")
	}

	installedVersion, parseErr := parseResticVersion(output)
	if parseErr != nil {
		return fmt.Errorf("parse wsl restic version: %w", parseErr)
	}

	if installedVersion != desiredVersion {
		return fmt.Errorf("wsl restic version mismatch: installed=%s required=%s; run backup setup", installedVersion, desiredVersion)
	}

	return nil
}

func syncWSLInteractive(ctx context.Context, desiredVersion string, exec system.Executor, confirm prompt.ConfirmFunc) (PlatformReport, error) {
	output, err := exec.RunCapture(ctx, "restic", "version")
	if err != nil {
		approved, confirmErr := confirm("WSL restic not found. Install via dnf now?")
		if confirmErr != nil {
			return PlatformReport{Platform: "wsl", Status: SetupFailed, Message: "prompt failed"}, confirmErr
		}
		if !approved {
			failure := fmt.Errorf("wsl restic is required")
			return PlatformReport{Platform: "wsl", Status: SetupFailed, Message: failure.Error()}, failure
		}
		if runErr := exec.Run(ctx, "sudo", "dnf", "install", "-y", "restic"); runErr != nil {
			return PlatformReport{Platform: "wsl", Status: SetupFailed, Message: "install failed"}, runErr
		}
		return PlatformReport{Platform: "wsl", Status: SetupInstalled, Message: "installed restic via dnf"}, nil
	}

	installedVersion, parseErr := parseResticVersion(output)
	if parseErr != nil {
		return PlatformReport{Platform: "wsl", Status: SetupFailed, Message: "version parse failed"}, fmt.Errorf("parse wsl restic version: %w", parseErr)
	}

	if installedVersion == desiredVersion {
		return PlatformReport{Platform: "wsl", Status: SetupMatched, Message: fmt.Sprintf("version %s matches", desiredVersion)}, nil
	}

	approved, confirmErr := confirm(fmt.Sprintf("WSL restic version is %s but config requires %s. Upgrade via dnf now?", installedVersion, desiredVersion))
	if confirmErr != nil {
		return PlatformReport{Platform: "wsl", Status: SetupFailed, Message: "prompt failed"}, confirmErr
	}
	if !approved {
		failure := fmt.Errorf("wsl restic version mismatch: installed=%s required=%s", installedVersion, desiredVersion)
		return PlatformReport{Platform: "wsl", Status: SetupFailed, Message: failure.Error()}, failure
	}

	if runErr := exec.Run(ctx, "sudo", "dnf", "upgrade", "-y", "restic"); runErr != nil {
		return PlatformReport{Platform: "wsl", Status: SetupFailed, Message: "upgrade failed"}, runErr
	}
	return PlatformReport{Platform: "wsl", Status: SetupUpgraded, Message: fmt.Sprintf("upgraded from %s to %s", installedVersion, desiredVersion)}, nil
}

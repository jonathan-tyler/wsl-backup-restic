package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/restic"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/system"
)

var loadWindowsProfilePassword = func(ctx context.Context) (string, error) {
	return restic.LoadPassword(ctx, os.Stdout, os.Stderr)
}

var osCreateTemp = os.CreateTemp

func executeWindowsProfileBackup(ctx context.Context, resticArgs []string, exec system.Executor) error {
	convertedArgs, err := convertRuleFileArgsToWindows(ctx, resticArgs, exec)
	if err != nil {
		return err
	}

	password := os.Getenv("RESTIC_PASSWORD")
	if strings.TrimSpace(password) == "" {
		password, err = loadWindowsProfilePassword(ctx)
		if err != nil {
			return err
		}
	}

	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("restic password is empty")
	}

	passwordFile, cleanup, err := createWindowsPasswordFile(ctx, password, exec)
	if err != nil {
		return err
	}
	defer cleanup()

	argsWithPassword := append([]string{"--password-file", passwordFile}, convertedArgs...)
	return exec.Run(ctx, "restic.exe", argsWithPassword...)
}

func createWindowsPasswordFile(ctx context.Context, password string, exec system.Executor) (string, func(), error) {
	file, err := osCreateTemp("", "wsl-backup-restic-password-*.txt")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temporary password file: %w", err)
	}

	cleanup := func() {
		_ = os.Remove(file.Name())
	}

	if _, err := file.WriteString(password + "\n"); err != nil {
		_ = file.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("write temporary password file: %w", err)
	}

	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("secure temporary password file permissions: %w", err)
	}

	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("close temporary password file: %w", err)
	}

	windowsPath, err := toWindowsPath(ctx, file.Name(), exec)
	if err != nil {
		cleanup()
		return "", func() {}, err
	}

	return windowsPath, cleanup, nil
}

func convertRuleFileArgsToWindows(ctx context.Context, args []string, exec system.Executor) ([]string, error) {
	converted := append([]string{}, args...)

	for index := 0; index < len(converted)-1; index++ {
		flag := converted[index]
		if flag != "--files-from" && flag != "--exclude-file" {
			continue
		}

		path := converted[index+1]
		if !looksLikeWSLPath(path) {
			continue
		}

		windowsPath, err := toWindowsPath(ctx, path, exec)
		if err != nil {
			return nil, err
		}
		converted[index+1] = windowsPath
	}

	return converted, nil
}

func looksLikeWSLPath(path string) bool {
	return strings.HasPrefix(path, "/") || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")
}

func toWindowsPath(ctx context.Context, path string, exec system.Executor) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path %q: %w", path, err)
	}

	output, err := exec.RunCapture(ctx, "wslpath", "-w", absPath)
	if err != nil {
		return "", fmt.Errorf("convert path %q to windows path: %w", path, err)
	}

	converted := strings.TrimSpace(output)
	if converted == "" {
		return "", fmt.Errorf("convert path %q to windows path: empty output", path)
	}
	return converted, nil
}

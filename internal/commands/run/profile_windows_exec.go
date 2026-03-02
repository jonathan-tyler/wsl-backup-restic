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

	return exec.RunWithEnv(ctx, map[string]string{"RESTIC_PASSWORD": password}, "restic.exe", convertedArgs...)
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

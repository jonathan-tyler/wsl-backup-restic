package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/restic"
	"github.com/jonathan-tyler/wsl-backup-restic/internal/system"
)

var loadWindowsProfilePassword = func(ctx context.Context) (string, error) {
	_ = ctx
	return restic.LoadPassword()
}

var resolveWindowsResticExecutable = func(ctx context.Context, exec system.Executor) (string, error) {
	output, err := exec.RunCapture(ctx, "pwsh.exe", "-NoProfile", "-Command", "(Get-Command restic.exe -ErrorAction Stop).Source")
	if err == nil {
		if path, ok := firstNonEmptyLine(output); ok {
			return path, nil
		}
	}

	output, err = exec.RunCapture(ctx, "where.exe", "restic.exe")
	if err == nil {
		if path, ok := firstNonEmptyLine(output); ok {
			return path, nil
		}
	}

	return "", fmt.Errorf("resolve windows restic executable path: restic.exe not found on PATH for the current Windows user")
}

var osCreateTemp = os.CreateTemp

const windowsTempDirWSL = "/mnt/c/Windows/Temp"

var elevatedWindowsTempDir = windowsTempDirWSL
var elevatedPathToWindows = wslPathToWindowsPath

func executeWindowsProfileBackup(ctx context.Context, resticArgs []string, runElevated bool, exec system.Executor) error {
	if runElevated {
		return runWindowsResticCommand(ctx, resticArgs, true, exec)
	}

	convertedArgs, err := convertRuleFileArgsToWindows(ctx, resticArgs, exec)
	if err != nil {
		return err
	}

	return runWindowsResticCommand(ctx, convertedArgs, false, exec)
}

func runWindowsResticCommand(ctx context.Context, resticArgs []string, runElevated bool, exec system.Executor) error {
	var err error
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

	if runElevated {
		elevatedArgs, cleanupRules, err := stageElevatedRuleFiles(resticArgs)
		if err != nil {
			return err
		}
		defer cleanupRules()

		passwordFile, cleanupPassword, err := createElevatedWindowsPasswordFile(password)
		if err != nil {
			return err
		}
		defer cleanupPassword()

		argsWithPassword := append([]string{"--password-file", passwordFile}, elevatedArgs...)

		resticExePath, err := resolveWindowsResticExecutable(ctx, exec)
		if err != nil {
			return err
		}
		return runElevatedWindowsRestic(ctx, resticExePath, argsWithPassword, exec)
	}

	passwordFile, cleanup, err := createWindowsPasswordFile(ctx, password, exec)
	if err != nil {
		return err
	}
	defer cleanup()

	argsWithPassword := append([]string{"--password-file", passwordFile}, resticArgs...)
	return exec.Run(ctx, "restic.exe", argsWithPassword...)
}

func runElevatedWindowsRestic(ctx context.Context, resticExePath string, args []string, exec system.Executor) error {
	wslExitCodeFile, windowsExitCodeFile, cleanup, err := createWindowsExitCodeFile()
	if err != nil {
		return err
	}
	defer cleanup()

	command := buildElevatedResticCommand(resticExePath, args, windowsExitCodeFile)
	if err := exec.Run(ctx, "pwsh.exe", "-NoProfile", "-Command", command); err != nil {
		return err
	}

	exitCode, err := readExitCodeFile(wslExitCodeFile)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("exit status %d", exitCode)
	}

	return nil
}

func buildElevatedResticCommand(resticExePath string, args []string, exitCodeFile string) string {
	escaped := make([]string, 0, len(args))
	for _, item := range args {
		escaped = append(escaped, singleQuotedPS(item))
	}

	inner := "& { & " + singleQuotedPS(resticExePath) + " @(" + strings.Join(escaped, ",") + "); $code = $LASTEXITCODE; Set-Content -LiteralPath " + singleQuotedPS(exitCodeFile) + " -Value $code -Encoding Ascii; exit $code }"

	return "$exitFile = " + singleQuotedPS(exitCodeFile) + "; $p = Start-Process -FilePath 'pwsh.exe' -ArgumentList @('-NoProfile','-Command'," + singleQuotedPS(inner) + ") -Verb RunAs -Wait -PassThru; if ($null -eq $p) { exit 1 }; exit 0"
}

func createWindowsExitCodeFile() (string, string, func(), error) {
	file, err := osCreateTemp(elevatedWindowsTempDir, "wsl-backup-restic-exitcode-*.txt")
	if err != nil {
		return "", "", func() {}, fmt.Errorf("create elevated exit-code file: %w", err)
	}

	wslPath := file.Name()
	cleanup := func() {
		_ = os.Remove(wslPath)
	}

	if err := file.Close(); err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("close elevated exit-code file: %w", err)
	}

	windowsPath, ok := elevatedPathToWindows(wslPath)
	if !ok {
		cleanup()
		return "", "", func() {}, fmt.Errorf("convert path %q to windows path: unsupported wsl mount path", wslPath)
	}

	return wslPath, windowsPath, cleanup, nil
}

func createElevatedWindowsPasswordFile(password string) (string, func(), error) {
	file, err := osCreateTemp(elevatedWindowsTempDir, "wsl-backup-restic-password-*.txt")
	if err != nil {
		return "", func() {}, fmt.Errorf("create elevated temporary password file: %w", err)
	}

	wslPath := file.Name()
	cleanup := func() {
		_ = os.Remove(wslPath)
	}

	if _, err := file.WriteString(password + "\n"); err != nil {
		_ = file.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("write elevated temporary password file: %w", err)
	}

	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("close elevated temporary password file: %w", err)
	}

	windowsPath, ok := elevatedPathToWindows(wslPath)
	if !ok {
		cleanup()
		return "", func() {}, fmt.Errorf("convert path %q to windows path: unsupported wsl mount path", wslPath)
	}

	return windowsPath, cleanup, nil
}

func stageElevatedRuleFiles(args []string) ([]string, func(), error) {
	converted := append([]string{}, args...)
	cleanupFns := make([]func(), 0)

	cleanup := func() {
		for i := len(cleanupFns) - 1; i >= 0; i-- {
			cleanupFns[i]()
		}
	}

	for index := 0; index < len(converted)-1; index++ {
		flag := converted[index]
		if flag != "--files-from" && flag != "--exclude-file" {
			continue
		}

		sourcePath := converted[index+1]
		if !looksLikeWSLPath(sourcePath) {
			continue
		}

		content, err := os.ReadFile(sourcePath)
		if err != nil {
			cleanup()
			return nil, func() {}, fmt.Errorf("read rule file %q for elevated windows backup: %w", sourcePath, err)
		}

		ext := filepath.Ext(sourcePath)
		file, err := osCreateTemp(elevatedWindowsTempDir, "wsl-backup-rule-*"+ext)
		if err != nil {
			cleanup()
			return nil, func() {}, fmt.Errorf("create elevated windows rule file: %w", err)
		}

		wslPath := file.Name()
		cleanupFns = append(cleanupFns, func() {
			_ = os.Remove(wslPath)
		})

		if _, err := file.Write(content); err != nil {
			_ = file.Close()
			cleanup()
			return nil, func() {}, fmt.Errorf("write elevated windows rule file: %w", err)
		}

		if err := file.Close(); err != nil {
			cleanup()
			return nil, func() {}, fmt.Errorf("close elevated windows rule file: %w", err)
		}

		windowsPath, ok := elevatedPathToWindows(wslPath)
		if !ok {
			cleanup()
			return nil, func() {}, fmt.Errorf("convert path %q to windows path: unsupported wsl mount path", wslPath)
		}

		converted[index+1] = windowsPath
	}

	return converted, cleanup, nil
}

func readExitCodeFile(path string) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read elevated exit-code file: %w", err)
	}

	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return 0, fmt.Errorf("read elevated exit-code file: empty content")
	}

	code, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("parse elevated exit-code file %q: %w", trimmed, err)
	}

	return code, nil
}

func singleQuotedPS(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func firstNonEmptyLine(value string) (string, bool) {
	for _, line := range strings.Split(value, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed, true
		}
	}

	return "", false
}

func wslPathToWindowsPath(path string) (string, bool) {
	if !strings.HasPrefix(path, "/mnt/") || len(path) < len("/mnt/c/") {
		return "", false
	}

	drive := path[len("/mnt/")]
	if !isASCIILetter(drive) {
		return "", false
	}

	rest := path[len("/mnt/c"):]
	rest = strings.ReplaceAll(rest, "/", `\\`)
	return strings.ToUpper(string(drive)) + ":" + rest, true
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

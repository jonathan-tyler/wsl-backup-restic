package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/restic"
	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/system"
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
	convertedArgs, cleanupRules, err := prepareWindowsRuleFiles(ctx, resticArgs, runElevated, exec)
	if err != nil {
		return err
	}
	defer cleanupRules()

	return runWindowsResticCommand(ctx, convertedArgs, runElevated, exec)
}

func runWindowsResticCommand(ctx context.Context, resticArgs []string, runElevated bool, exec system.Executor) error {
	convertedArgs, err := prepareWindowsRepositoryArgs(ctx, resticArgs, exec)
	if err != nil {
		return err
	}
	resticArgs = convertedArgs

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
		passwordFile, cleanupPassword, err := createElevatedWindowsPasswordFile(password)
		if err != nil {
			return err
		}
		defer cleanupPassword()

		argsWithPassword := append([]string{"--password-file", passwordFile}, resticArgs...)

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

func prepareWindowsRepositoryArgs(ctx context.Context, args []string, exec system.Executor) ([]string, error) {
	converted := append([]string{}, args...)

	for index := 0; index < len(converted)-1; index++ {
		if converted[index] != "--repo" {
			continue
		}

		repository := converted[index+1]
		if !looksLikeWSLWindowsMountPath(repository) {
			continue
		}

		windowsPath, err := toWindowsPath(ctx, repository, exec)
		if err != nil {
			return nil, fmt.Errorf("convert repository path %q to windows path: %w", repository, err)
		}

		converted[index+1] = windowsPath
	}

	return converted, nil
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
	file, err := osCreateTemp(elevatedWindowsTempDir, "wsl-backup-orchestrator-exitcode-*.txt")
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
	file, err := osCreateTemp(elevatedWindowsTempDir, "wsl-backup-orchestrator-password-*.txt")
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

func prepareWindowsRuleFiles(ctx context.Context, args []string, runElevated bool, exec system.Executor) ([]string, func(), error) {
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

		windowsPath, cleanupRule, err := stageRuleFileForWindows(ctx, sourcePath, flag, runElevated, exec)
		if err != nil {
			cleanup()
			return nil, func() {}, err
		}

		cleanupFns = append(cleanupFns, cleanupRule)
		converted[index+1] = windowsPath
	}

	return converted, cleanup, nil
}

func stageRuleFileForWindows(ctx context.Context, sourcePath string, flag string, runElevated bool, exec system.Executor) (string, func(), error) {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", func() {}, fmt.Errorf("read rule file %q for windows backup: %w", sourcePath, err)
	}

	translatedContent := translateRuleFileContentForWindows(content, flag)
	ext := filepath.Ext(sourcePath)
	tempDir := ""
	if runElevated {
		tempDir = elevatedWindowsTempDir
	}

	file, err := osCreateTemp(tempDir, "wsl-backup-orchestrator-rule-*"+ext)
	if err != nil {
		if runElevated {
			return "", func() {}, fmt.Errorf("create elevated windows rule file: %w", err)
		}
		return "", func() {}, fmt.Errorf("create windows rule file: %w", err)
	}

	wslPath := file.Name()
	cleanup := func() {
		_ = os.Remove(wslPath)
	}

	if _, err := file.Write(translatedContent); err != nil {
		_ = file.Close()
		cleanup()
		if runElevated {
			return "", func() {}, fmt.Errorf("write elevated windows rule file: %w", err)
		}
		return "", func() {}, fmt.Errorf("write windows rule file: %w", err)
	}

	if err := file.Close(); err != nil {
		cleanup()
		if runElevated {
			return "", func() {}, fmt.Errorf("close elevated windows rule file: %w", err)
		}
		return "", func() {}, fmt.Errorf("close windows rule file: %w", err)
	}

	if runElevated {
		windowsPath, ok := elevatedPathToWindows(wslPath)
		if !ok {
			cleanup()
			return "", func() {}, fmt.Errorf("convert path %q to windows path: unsupported wsl mount path", wslPath)
		}
		return windowsPath, cleanup, nil
	}

	windowsPath, err := toWindowsPath(ctx, wslPath, exec)
	if err != nil {
		cleanup()
		return "", func() {}, err
	}

	return windowsPath, cleanup, nil
}

func translateRuleFileContentForWindows(content []byte, flag string) []byte {
	lines := strings.Split(string(content), "\n")
	filterLinuxOnlyPaths := flag == "--files-from"
	for index, line := range lines {
		lines[index] = translateRuleLineForWindows(line, filterLinuxOnlyPaths)
	}
	return []byte(strings.Join(lines, "\n"))
}

func translateRuleLineForWindows(line string, filterLinuxOnlyPaths bool) string {
	trimmedLeft := strings.TrimLeft(line, " \t")
	if trimmedLeft == "" || strings.HasPrefix(trimmedLeft, "#") {
		return line
	}

	leading := line[:len(line)-len(trimmedLeft)]
	if translated, ok := wslPathToWindowsPath(trimmedLeft); ok {
		return leading + translated
	}
	if filterLinuxOnlyPaths && looksLikeLinuxAbsolutePath(trimmedLeft) && !looksLikeWindowsPath(trimmedLeft) {
		return ""
	}

	return line
}

func looksLikeLinuxAbsolutePath(value string) bool {
	return strings.HasPrefix(value, "/")
}

func looksLikeWindowsPath(value string) bool {
	if len(value) < 3 {
		return false
	}
	return isASCIILetter(value[0]) && value[1] == ':' && (value[2] == '\\' || value[2] == '/')
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
	rest = strings.ReplaceAll(rest, "/", `\`)
	return strings.ToUpper(string(drive)) + ":" + rest, true
}

func createWindowsPasswordFile(ctx context.Context, password string, exec system.Executor) (string, func(), error) {
	file, err := osCreateTemp("", "wsl-backup-orchestrator-password-*.txt")
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

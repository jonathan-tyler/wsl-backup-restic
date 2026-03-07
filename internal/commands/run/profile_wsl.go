package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jonathan-tyler/wsl-backup-orchestrator/internal/restic"
)

func executeWSLProfileBackup(ctx context.Context, resticArgs []string, runner restic.Executor) error {
	convertedArgs, cleanupRules, err := prepareWSLRuleFiles(resticArgs)
	if err != nil {
		return err
	}
	defer cleanupRules()

	return runner.Run(ctx, convertedArgs...)
}

func prepareWSLRuleFiles(args []string) ([]string, func(), error) {
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
		stagedPath, cleanupRule, err := stageRuleFileForWSL(sourcePath, flag)
		if err != nil {
			cleanup()
			return nil, func() {}, err
		}

		cleanupFns = append(cleanupFns, cleanupRule)
		converted[index+1] = stagedPath
	}

	return converted, cleanup, nil
}

func stageRuleFileForWSL(sourcePath string, flag string) (string, func(), error) {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", func() {}, fmt.Errorf("read rule file %q for wsl backup: %w", sourcePath, err)
	}

	translatedContent := translateRuleFileContentForWSL(content, flag)
	if string(translatedContent) == string(content) {
		return sourcePath, func() {}, nil
	}

	file, err := osCreateTemp("", "wsl-backup-orchestrator-rule-*"+filepath.Ext(sourcePath))
	if err != nil {
		return "", func() {}, fmt.Errorf("create wsl rule file: %w", err)
	}

	wslPath := file.Name()
	cleanup := func() {
		_ = os.Remove(wslPath)
	}

	if _, err := file.Write(translatedContent); err != nil {
		_ = file.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("write wsl rule file: %w", err)
	}

	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("close wsl rule file: %w", err)
	}

	return wslPath, cleanup, nil
}

func translateRuleFileContentForWSL(content []byte, flag string) []byte {
	lines := strings.Split(string(content), "\n")
	filterWindowsPaths := flag == "--files-from"
	for index, line := range lines {
		lines[index] = translateRuleLineForWSL(line, filterWindowsPaths)
	}
	return []byte(strings.Join(lines, "\n"))
}

func translateRuleLineForWSL(line string, filterWindowsPaths bool) string {
	trimmedLeft := strings.TrimLeft(line, " \t")
	if trimmedLeft == "" || strings.HasPrefix(trimmedLeft, "#") {
		return line
	}

	if filterWindowsPaths && (looksLikeWSLWindowsMountPath(trimmedLeft) || looksLikeWindowsPath(trimmedLeft)) {
		return ""
	}

	return line
}

func looksLikeWSLWindowsMountPath(path string) bool {
	if !strings.HasPrefix(path, "/mnt/") || len(path) < len("/mnt/c/") {
		return false
	}

	drive := path[len("/mnt/")]
	if !isASCIILetter(drive) {
		return false
	}

	return path[len("/mnt/c")] == '/'
}

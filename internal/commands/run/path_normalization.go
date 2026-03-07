package run

import (
	"path/filepath"
	"strings"
)

func normalizePath(value string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	normalized = normalizeWSLMountPath(normalized)
	normalized = strings.TrimSuffix(normalized, "/")
	normalized = filepath.Clean(normalized)
	if len(normalized) >= 2 && normalized[1] == ':' {
		normalized = strings.ToLower(normalized)
	}
	return normalized
}

func normalizeWSLMountPath(value string) string {
	if !strings.HasPrefix(value, "/mnt/") || len(value) < len("/mnt/c") {
		return value
	}

	drive := value[len("/mnt/")]
	if !isASCIILetter(drive) {
		return value
	}

	if len(value) == len("/mnt/c") {
		return strings.ToLower(string(drive)) + ":/"
	}

	if value[len("/mnt/c")] != '/' {
		return value
	}

	return strings.ToLower(string(drive)) + ":" + value[len("/mnt/c"):]
}

func isASCIILetter(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}

package restic

import (
	"fmt"
	"os"
	"strings"
)

const (
	ResticPasswordFileEnv      = "RESTIC_PASSWORD_FILE"
	WSLBackupPasswordFileEnv   = "WSL_BACKUP_RESTIC_PASSWORD_FILE"
	SystemdCredentialsDirEnv   = "CREDENTIALS_DIRECTORY"
	SystemdResticPasswordCred  = "restic_password"
)

func loadResticPassword() (string, error) {
	if envPassword := strings.TrimSpace(os.Getenv("RESTIC_PASSWORD")); envPassword != "" {
		return envPassword, nil
	}

	passwordFileCandidates := []string{
		strings.TrimSpace(os.Getenv(WSLBackupPasswordFileEnv)),
		strings.TrimSpace(os.Getenv(ResticPasswordFileEnv)),
	}

	if credentialsDir := strings.TrimSpace(os.Getenv(SystemdCredentialsDirEnv)); credentialsDir != "" {
		passwordFileCandidates = append(passwordFileCandidates, credentialsDir+"/"+SystemdResticPasswordCred)
	}

	for _, candidate := range passwordFileCandidates {
		if candidate == "" {
			continue
		}
		password, err := readPasswordFromFile(candidate)
		if err != nil {
			return "", err
		}
		if password != "" {
			return password, nil
		}
	}

	return "", fmt.Errorf("restic password is not configured: set RESTIC_PASSWORD, %s, %s, or provide systemd credential %q", WSLBackupPasswordFileEnv, ResticPasswordFileEnv, SystemdResticPasswordCred)
}

func CheckPasswordConfigured() error {
	_, err := loadResticPassword()
	return err
}

func LoadPassword() (string, error) {
	return loadResticPassword()
}

func readPasswordFromFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read restic password file %q: %w", path, err)
	}

	password := strings.TrimSpace(string(content))
	if password == "" {
		return "", fmt.Errorf("restic password file %q is empty", path)
	}

	return password, nil
}

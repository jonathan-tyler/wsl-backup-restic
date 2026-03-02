package restic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadResticPasswordFromPreferredFileEnv(t *testing.T) {
	t.Setenv("RESTIC_PASSWORD", "")

	dir := t.TempDir()
	passwordPath := filepath.Join(dir, "pass.txt")
	if err := os.WriteFile(passwordPath, []byte("abc123\n"), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}
	t.Setenv(WSLBackupPasswordFileEnv, passwordPath)
	t.Setenv(ResticPasswordFileEnv, "")
	t.Setenv(SystemdCredentialsDirEnv, "")

	password, err := loadResticPassword()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if password != "abc123" {
		t.Fatalf("unexpected password value: %q", password)
	}
}

func TestLoadResticPasswordFromSystemdCredential(t *testing.T) {
	t.Setenv("RESTIC_PASSWORD", "")
	t.Setenv(WSLBackupPasswordFileEnv, "")
	t.Setenv(ResticPasswordFileEnv, "")

	dir := t.TempDir()
	credPath := filepath.Join(dir, SystemdResticPasswordCred)
	if err := os.WriteFile(credPath, []byte("from-cred\n"), 0o600); err != nil {
		t.Fatalf("write credential file: %v", err)
	}
	t.Setenv(SystemdCredentialsDirEnv, dir)

	password, err := loadResticPassword()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if password != "from-cred" {
		t.Fatalf("unexpected password value: %q", password)
	}
}

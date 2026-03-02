package restic

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatCommandQuotesWhitespace(t *testing.T) {
	formatted := formatCommand([]string{"backup", "--target", "/tmp/my folder"})
	if formatted != "backup --target \"/tmp/my folder\"" {
		t.Fatalf("unexpected format: %s", formatted)
	}
}

func TestOSRunnerPrintsCommandAndStreamsOutput(t *testing.T) {
	resetPasswordCacheForTest()
	original := commandContext
	originalLookPath := commandLookPath
	commandContext = fakeExecCommand
	commandLookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() {
		commandContext = original
		commandLookPath = originalLookPath
	})

	configPath := writeConfigFile(t, `restic_version: "0.18.1"
keepassxc_database: /tmp/vault.kdbx
keepassxc_entry: restic/main
profiles:
  wsl:
    repository: /repo/wsl
    use_fs_snapshot: false
`)
	t.Setenv("BACKUP_CONFIG", configPath)
	t.Setenv(KeepassDatabaseEnv, "")
	t.Setenv(KeepassEntryEnv, "")

	var stdout strings.Builder
	var stderr strings.Builder
	runner := NewOSRunner(&stdout, &stderr)

	err := runner.Run(context.Background(), "snapshots")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "$ keepassxc-cli show -q -a Password /tmp/vault.kdbx restic/main") {
		t.Fatalf("expected keepass lookup command in stdout, got %q", out)
	}
	if !strings.Contains(out, "$ restic snapshots") {
		t.Fatalf("expected echoed command in stdout, got %q", out)
	}
	if !strings.Contains(out, "helper stdout") {
		t.Fatalf("expected command stdout in stdout writer, got %q", out)
	}
	if !strings.Contains(stderr.String(), "helper stderr") {
		t.Fatalf("expected command stderr in stderr writer, got %q", stderr.String())
	}
}

func TestOSRunnerRejectsEmptyArgs(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	runner := NewOSRunner(&stdout, &stderr)
	err := runner.Run(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestOSRunnerReadsKeepassSettingsFromEnvOverrides(t *testing.T) {
	resetPasswordCacheForTest()
	original := commandContext
	originalLookPath := commandLookPath
	commandContext = fakeExecCommand
	commandLookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() {
		commandContext = original
		commandLookPath = originalLookPath
	})

	configPath := writeConfigFile(t, `restic_version: "0.18.1"
keepassxc_database: /tmp/config-vault.kdbx
keepassxc_entry: config/restic
profiles:
  wsl:
    repository: /repo/wsl
    use_fs_snapshot: false
`)
	t.Setenv("BACKUP_CONFIG", configPath)
	t.Setenv(KeepassDatabaseEnv, "/tmp/env-vault.kdbx")
	t.Setenv(KeepassEntryEnv, "env/restic")

	var stdout strings.Builder
	var stderr strings.Builder
	runner := NewOSRunner(&stdout, &stderr)

	err := runner.Run(context.Background(), "snapshots")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "$ keepassxc-cli show -q -a Password /tmp/env-vault.kdbx env/restic") {
		t.Fatalf("expected env override keepass command, got %q", out)
	}
}

func TestOSRunnerFailsWhenKeepassSettingsMissing(t *testing.T) {
	resetPasswordCacheForTest()
	original := commandContext
	originalLookPath := commandLookPath
	commandContext = fakeExecCommand
	commandLookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() {
		commandContext = original
		commandLookPath = originalLookPath
	})

	configPath := writeConfigFile(t, `restic_version: "0.18.1"
profiles:
  wsl:
    repository: /repo/wsl
    use_fs_snapshot: false
`)
	t.Setenv("BACKUP_CONFIG", configPath)
	t.Setenv(KeepassDatabaseEnv, "")
	t.Setenv(KeepassEntryEnv, "")

	var stdout strings.Builder
	var stderr strings.Builder
	runner := NewOSRunner(&stdout, &stderr)

	err := runner.Run(context.Background(), "snapshots")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "missing KeepassXC lookup settings") {
		t.Fatalf("expected missing settings error, got %v", err)
	}
}

func TestOSRunnerFailsWhenKeepassLookupFails(t *testing.T) {
	resetPasswordCacheForTest()
	original := commandContext
	originalLookPath := commandLookPath
	commandContext = fakeExecCommand
	commandLookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() {
		commandContext = original
		commandLookPath = originalLookPath
	})

	t.Setenv(KeepassDatabaseEnv, "/tmp/vault.kdbx")
	t.Setenv(KeepassEntryEnv, "restic/main")
	t.Setenv("FAKE_KEEPASS_FAIL", "1")

	var stdout strings.Builder
	var stderr strings.Builder
	runner := NewOSRunner(&stdout, &stderr)

	err := runner.Run(context.Background(), "snapshots")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "keepassxc-cli password lookup failed") {
		t.Fatalf("expected keepass lookup error, got %v", err)
	}
}

func TestWithResticPasswordReplacesExistingValue(t *testing.T) {
	base := []string{"PATH=/usr/bin", "RESTIC_PASSWORD=old", "HOME=/tmp"}
	updated := withResticPassword(base, "new-secret")

	if strings.Count(strings.Join(updated, "|"), "RESTIC_PASSWORD=") != 1 {
		t.Fatalf("expected exactly one RESTIC_PASSWORD entry, got %#v", updated)
	}
	if updated[len(updated)-1] != "RESTIC_PASSWORD=new-secret" {
		t.Fatalf("expected appended replacement password, got %#v", updated)
	}
}

func TestOSRunnerFailsWhenKeepassReturnsEmptyPassword(t *testing.T) {
	resetPasswordCacheForTest()
	original := commandContext
	originalLookPath := commandLookPath
	commandContext = fakeExecCommand
	commandLookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() {
		commandContext = original
		commandLookPath = originalLookPath
	})

	t.Setenv(KeepassDatabaseEnv, "/tmp/vault.kdbx")
	t.Setenv(KeepassEntryEnv, "restic/main")
	t.Setenv("FAKE_KEEPASS_EMPTY", "1")

	var stdout strings.Builder
	var stderr strings.Builder
	runner := NewOSRunner(&stdout, &stderr)

	err := runner.Run(context.Background(), "snapshots")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "returned an empty password") {
		t.Fatalf("expected empty-password error, got %v", err)
	}
}

func TestResolveKeepassLookupSettingsUsesConfig(t *testing.T) {
	resetPasswordCacheForTest()
	configPath := writeConfigFile(t, `restic_version: "0.18.1"
keepassxc_database: /tmp/config-vault.kdbx
keepassxc_entry: config/restic
profiles:
  wsl:
    repository: /repo/wsl
    use_fs_snapshot: false
`)
	t.Setenv("BACKUP_CONFIG", configPath)
	t.Setenv(KeepassDatabaseEnv, "")
	t.Setenv(KeepassEntryEnv, "")

	database, entry, err := resolveKeepassLookupSettings()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if database != "/tmp/config-vault.kdbx" || entry != "config/restic" {
		t.Fatalf("unexpected lookup settings: database=%q entry=%q", database, entry)
	}
}

func TestOSRunnerUsesFlatpakFallbackForKeepassCLI(t *testing.T) {
	resetPasswordCacheForTest()
	original := commandContext
	originalLookPath := commandLookPath
	commandContext = fakeExecCommand
	commandLookPath = func(name string) (string, error) {
		if name == "keepassxc-cli" {
			return "", fmt.Errorf("not found")
		}
		if name == "flatpak" {
			return "/usr/bin/flatpak", nil
		}
		return "", fmt.Errorf("not found")
	}
	t.Cleanup(func() {
		commandContext = original
		commandLookPath = originalLookPath
	})

	t.Setenv(KeepassDatabaseEnv, "/tmp/vault.kdbx")
	t.Setenv(KeepassEntryEnv, "restic/main")

	var stdout strings.Builder
	var stderr strings.Builder
	runner := NewOSRunner(&stdout, &stderr)

	err := runner.Run(context.Background(), "snapshots")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "$ flatpak run --command=keepassxc-cli org.keepassxc.KeePassXC show -q -a Password /tmp/vault.kdbx restic/main") {
		t.Fatalf("expected flatpak fallback command in stdout, got %q", out)
	}
}

func TestCheckKeepassCLIAvailableFailsWhenNoCommand(t *testing.T) {
	originalLookPath := commandLookPath
	commandLookPath = func(string) (string, error) { return "", fmt.Errorf("not found") }
	t.Cleanup(func() {
		commandLookPath = originalLookPath
	})

	err := CheckKeepassCLIAvailable()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "flatpak fallback") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveKeepassLookupSettingsFailsWithPartialEnvAndMissingConfig(t *testing.T) {
	t.Setenv("BACKUP_CONFIG", "/tmp/does-not-exist-config.yaml")
	t.Setenv(KeepassDatabaseEnv, "/tmp/env-vault.kdbx")
	t.Setenv(KeepassEntryEnv, "")

	_, _, err := resolveKeepassLookupSettings()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "unable to complete KeepassXC lookup settings from config") {
		t.Fatalf("expected partial-env config-completion error, got %v", err)
	}
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}

func fakeExecCommand(_ context.Context, name string, args ...string) *exec.Cmd {
	allArgs := []string{"-test.run=TestHelperProcess", "--"}
	allArgs = append(allArgs, name)
	allArgs = append(allArgs, args...)
	cmd := exec.Command(os.Args[0], allArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	if len(os.Args) < 4 {
		os.Exit(2)
	}

	commandName := os.Args[3]
	if commandName == "keepassxc-cli" || commandName == "flatpak" {
		if os.Getenv("FAKE_KEEPASS_FAIL") == "1" {
			fmt.Fprintln(os.Stderr, "database is locked")
			os.Exit(1)
		}
		if os.Getenv("FAKE_KEEPASS_EMPTY") == "1" {
			fmt.Fprintln(os.Stdout)
			os.Exit(0)
		}

		fmt.Fprintln(os.Stdout, "test-password")
		os.Exit(0)
	}

	if commandName == "restic" {
		if os.Getenv("RESTIC_PASSWORD") == "" {
			fmt.Fprintln(os.Stderr, "missing RESTIC_PASSWORD")
			os.Exit(2)
		}

		fmt.Fprintln(os.Stdout, "helper stdout")
		fmt.Fprintln(os.Stderr, "helper stderr")
		os.Exit(0)
	}

	fmt.Fprintln(os.Stderr, "unknown helper command")
	os.Exit(2)
}

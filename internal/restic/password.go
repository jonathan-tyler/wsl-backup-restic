package restic

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/jonathan-tyler/wsl-backup-restic/internal/config"
	"gopkg.in/yaml.v3"
)

const (
	KeepassDatabaseEnv = "WSL_BACKUP_KEEPASSXC_DATABASE"
	KeepassEntryEnv    = "WSL_BACKUP_KEEPASSXC_ENTRY"
)

var commandLookPath = exec.LookPath

type keepassCLICommand struct {
	name string
	args []string
}

func loadResticPassword(ctx context.Context, stdout io.Writer, stderr io.Writer) (string, error) {
	database, entry, err := resolveKeepassLookupSettings()
	if err != nil {
		return "", err
	}

	keepassCmd, err := resolveKeepassCLICommand()
	if err != nil {
		return "", err
	}

	args := []string{"show", "-q", "-a", "Password", database, entry}
	commandArgs := append(append([]string{}, keepassCmd.args...), args...)
	printed := append([]string{keepassCmd.name}, commandArgs...)
	fmt.Fprintf(stdout, "$ %s\n", formatCommand(printed))

	cmd := commandContext(ctx, keepassCmd.name, commandArgs...)
	cmd.Stderr = stderr

	secret, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("keepassxc-cli password lookup failed (is database unlocked?): %w", err)
	}

	password := strings.TrimSpace(string(secret))
	if password == "" {
		return "", fmt.Errorf("keepassxc-cli returned an empty password")
	}

	return password, nil
}

func CheckKeepassCLIAvailable() error {
	_, err := resolveKeepassCLICommand()
	return err
}

func resolveKeepassCLICommand() (keepassCLICommand, error) {
	if _, err := commandLookPath("keepassxc-cli"); err == nil {
		return keepassCLICommand{name: "keepassxc-cli"}, nil
	}

	if _, err := commandLookPath("flatpak"); err == nil {
		return keepassCLICommand{
			name: "flatpak",
			args: []string{"run", "--command=keepassxc-cli", "org.keepassxc.KeePassXC"},
		}, nil
	}

	return keepassCLICommand{}, fmt.Errorf("keepassxc-cli is not available in PATH and flatpak fallback is unavailable")
}

func resolveKeepassLookupSettings() (string, string, error) {
	envDatabase := strings.TrimSpace(os.Getenv(KeepassDatabaseEnv))
	envEntry := strings.TrimSpace(os.Getenv(KeepassEntryEnv))
	if envDatabase != "" && envEntry != "" {
		return envDatabase, envEntry, nil
	}

	cfgDatabase, cfgEntry, err := loadKeepassLookupSettingsFromConfig()
	if err != nil {
		if envDatabase != "" || envEntry != "" {
			return "", "", fmt.Errorf("unable to complete KeepassXC lookup settings from config: %w", err)
		}
		return "", "", fmt.Errorf("unable to load KeepassXC lookup settings from config: %w", err)
	}

	database := cfgDatabase
	if envDatabase != "" {
		database = envDatabase
	}

	entry := cfgEntry
	if envEntry != "" {
		entry = envEntry
	}

	if database == "" || entry == "" {
		return "", "", fmt.Errorf(
			"missing KeepassXC lookup settings: set keepassxc_database and keepassxc_entry in config, or set %s and %s",
			KeepassDatabaseEnv,
			KeepassEntryEnv,
		)
	}

	return database, entry, nil
}

type keepassLookupConfig struct {
	KeepassDB    string `yaml:"keepassxc_database"`
	KeepassEntry string `yaml:"keepassxc_entry"`
}

func loadKeepassLookupSettingsFromConfig() (string, string, error) {
	loader := config.NewLoader()
	path, err := loader.ResolvePath()
	if err != nil {
		return "", "", err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg keepassLookupConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return "", "", fmt.Errorf("parse config %q: %w", path, err)
	}

	return strings.TrimSpace(cfg.KeepassDB), strings.TrimSpace(cfg.KeepassEntry), nil
}

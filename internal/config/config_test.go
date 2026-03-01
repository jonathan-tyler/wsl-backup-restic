package config

import (
	"errors"
	"strings"
	"testing"
)

func TestResolvePathUsesOverride(t *testing.T) {
	loader := Loader{Getenv: func(key string) string {
		if key == "BACKUP_CONFIG" {
			return "/tmp/custom.yaml"
		}
		return ""
	}}

	path, err := loader.ResolvePath()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if path != "/tmp/custom.yaml" {
		t.Fatalf("expected override path, got %q", path)
	}
}

func TestResolvePathUsesXDGConfig(t *testing.T) {
	loader := Loader{Getenv: func(key string) string {
		switch key {
		case "XDG_CONFIG_HOME":
			return "/home/me/.cfg"
		default:
			return ""
		}
	}}

	path, err := loader.ResolvePath()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if path != "/home/me/.cfg/wsl-backup/config.yaml" {
		t.Fatalf("unexpected path: %q", path)
	}
}

func TestResolvePathUsesHomeFallback(t *testing.T) {
	loader := Loader{Getenv: func(key string) string {
		if key == "HOME" {
			return "/home/me"
		}
		return ""
	}}

	path, err := loader.ResolvePath()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if path != "/home/me/.config/wsl-backup/config.yaml" {
		t.Fatalf("unexpected path: %q", path)
	}
}

func TestResolvePathFailsWithoutHome(t *testing.T) {
	loader := Loader{Getenv: func(string) string { return "" }}

	_, err := loader.ResolvePath()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadParsesConfig(t *testing.T) {
	loader := Loader{
		Getenv: func(key string) string {
			if key == "BACKUP_CONFIG" {
				return "/tmp/config.yaml"
			}
			return ""
		},
		ReadFile: func(path string) ([]byte, error) {
			if path != "/tmp/config.yaml" {
				t.Fatalf("unexpected path: %s", path)
			}
			return []byte(`restic_version: "0.18.1"
profiles:
  wsl:
    repository: /repo/wsl
    use_fs_snapshot: false
`), nil
		},
	}

	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if cfg.Path() != "/tmp/config.yaml" {
		t.Fatalf("unexpected config path: %q", cfg.Path())
	}
	if cfg.Profiles["wsl"].Repository != "/repo/wsl" {
		t.Fatalf("unexpected repository")
	}
}

func TestLoadFailsWhenProfileRepositoryMissing(t *testing.T) {
	loader := Loader{
		Getenv: func(key string) string {
			if key == "BACKUP_CONFIG" {
				return "/tmp/config.yaml"
			}
			return ""
		},
		ReadFile: func(string) ([]byte, error) {
			return []byte(`profiles:
  wsl:
    repository: ""
`), nil
		},
	}

	_, err := loader.Load()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "empty repository") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadWrapsReadError(t *testing.T) {
	loader := Loader{
		Getenv: func(key string) string {
			if key == "BACKUP_CONFIG" {
				return "/tmp/config.yaml"
			}
			return ""
		},
		ReadFile: func(string) ([]byte, error) {
			return nil, errors.New("read fail")
		},
	}

	_, err := loader.Load()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "read fail") {
		t.Fatalf("unexpected error: %v", err)
	}
}

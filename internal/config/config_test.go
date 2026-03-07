package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileDirAndPath(t *testing.T) {
	var cfg File
	if cfg.Path() != "" {
		t.Fatalf("expected empty path")
	}
	if cfg.Dir() != "" {
		t.Fatalf("expected empty dir")
	}

	withPath := FileWithPathForTest(cfg, "/tmp/wsl-backup/config.yaml")
	if withPath.Path() != "/tmp/wsl-backup/config.yaml" {
		t.Fatalf("unexpected path: %q", withPath.Path())
	}
	if withPath.Dir() != "/tmp/wsl-backup" {
		t.Fatalf("unexpected dir: %q", withPath.Dir())
	}
}

func TestIncludeAndExcludeRulesPath(t *testing.T) {
	include := IncludeRulesPath("/tmp/wsl-backup", "wsl", "daily")
	exclude := ExcludeRulesPath("/tmp/wsl-backup")

	if include != filepath.Join("/tmp/wsl-backup", "includes.daily.txt") {
		t.Fatalf("unexpected include path: %q", include)
	}
	if exclude != filepath.Join("/tmp/wsl-backup", "excludes.txt") {
		t.Fatalf("unexpected exclude path: %q", exclude)
	}
}

func TestNewLoaderProvidesFunctions(t *testing.T) {
	loader := NewLoader()
	if loader.ReadFile == nil {
		t.Fatalf("expected ReadFile function")
	}
	if loader.Getenv == nil {
		t.Fatalf("expected Getenv function")
	}
}

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
  windows:
    repository: /mnt/c/repo/windows
    use_fs_snapshot: true
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
	if !cfg.Profiles["windows"].UseFSSnapshot {
		t.Fatalf("expected windows use_fs_snapshot to be true")
	}
}

func TestLoadFailsWhenWSLUsesFSSnapshot(t *testing.T) {
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
    repository: /repo/wsl
    use_fs_snapshot: true
`), nil
		},
	}

	_, err := loader.Load()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "supported only for the windows profile") {
		t.Fatalf("unexpected error: %v", err)
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

func TestLoadWrapsParseError(t *testing.T) {
	loader := Loader{
		Getenv: func(key string) string {
			if key == "BACKUP_CONFIG" {
				return "/tmp/config.yaml"
			}
			return ""
		},
		ReadFile: func(string) ([]byte, error) {
			return []byte("profiles: ["), nil
		},
	}

	_, err := loader.Load()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadFailsWhenNoProfiles(t *testing.T) {
	loader := Loader{
		Getenv: func(key string) string {
			if key == "BACKUP_CONFIG" {
				return "/tmp/config.yaml"
			}
			return ""
		},
		ReadFile: func(string) ([]byte, error) {
			return []byte(`restic_version: "0.18.1"
profiles: {}
`), nil
		},
	}

	_, err := loader.Load()
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "config has no profiles") {
		t.Fatalf("unexpected error: %v", err)
	}
}

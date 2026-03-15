# WSL Backup Orchestrator

WSL-centric backup orchestration CLI for Windows and WSL filesystems, using [restic](https://restic.github.io/).

## What it does

- Runs from WSL and concurrently performs cross-platform backups for WSL and Windows
- Supports *cadences* of `daily`, `weekly`, and `monthly` for backup and reporting commands
- Ensures match `restic` versions on WSL (via `dnf`) and Windows (via `scoop`)

## Configuration

- Copy [config.example.yaml](config.example.yaml) to `~/.config/wsl-backup/config.yaml` and customize as needed.
- Each cadence-based "includes" rule file inherits from the previous cadence(s) and is cumulative
- All paths are expected to be from the WSL path perspective

### Authentication

- Restic password is a hard requirement.
- Supported password sources (first available wins):
  - `RESTIC_PASSWORD` (direct value)
  - `RESTIC_PASSWORD_FILE` (restic-compatible password file path)
  - systemd credentials directory (`$CREDENTIALS_DIRECTORY/restic_password`)
  - Manual `wsl-backup run ...` invocations via prompt once

## Usage

The CLI is WSL-only; run it from a WSL shell.

Typical flow:

```sh
# One-time (or when versions drift)
wsl-backup setup

# Regular operation
wsl-backup run daily
wsl-backup run weekly
wsl-backup run monthly
wsl-backup run daily --dry-run

# Restore examples
wsl-backup restore /tmp/restore-target
wsl-backup restore /tmp/restore-target --dry-run
```

## Build and Install

The repo includes containerized build helpers that use rootless `podman` by default.

```sh
./scripts/build-in-container.sh
./scripts/install.sh
```

- `build-in-container.sh` builds `out/wsl-backup` inside a Go container.
- `install.sh` runs the same containerized build, installs the binary to `${HOME}/.local/bin/wsl-backup` by default, and scaffolds `${XDG_CONFIG_HOME:-$HOME/.config}/wsl-backup` if files are missing.
- The installer copies `config.example.yaml` to `config.yaml` only when that file does not already exist, and it creates missing empty `includes.daily.txt`, `includes.weekly.txt`, `includes.monthly.txt`, and `excludes.txt` files without overwriting existing content.
- Override `CONTAINER_ENGINE`, `GOOS`, `GOARCH`, `OUT_DIR`, `PREFIX`, `BINDIR`, or `CONFIG_DIR` when needed.

## Caveats

- `restic` stores symlinks as symlinks by default and does not follow them during backup. This behavior helps avoid recursive traversal from link loops. If symlink following is enabled explicitly in a restic invocation, traversal/loop risk must be evaluated separately.
- `use_fs_snapshot` is supported only for the `windows` profile.
- On the Windows profile, enabling `use_fs_snapshot` passes `--use-fs-snapshot` to `restic` and automatically launches `restic.exe` through an elevated UAC prompt (`Start-Process -Verb RunAs`). This uses Volume Shadow Copy Service (VSS) snapshots for each volume involved in the backup, so data is read from snapshots instead of the live filesystem and can include files locked by running processes.

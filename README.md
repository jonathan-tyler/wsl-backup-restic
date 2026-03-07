# WSL Backup Orchestrator

Thin, predictable wrapper around `restic`, run from WSL and usable as a `wsl-sys-cli` extension.

## What it does

- Runs from WSL and targets cross-platform backup flows (WSL + Windows)
- Runs configured profile backups concurrently after preflight completes
- Prints `restic snapshots` for each configured repository after all backups finish successfully
- Checks for matching `restic` versions on WSL (via `dnf`) and Windows (via `scoop`), and offers to install/upgrade when mismatched or missing
- Supports *cadences* of `daily`, `weekly`, and `monthly` for backup and reporting commands

## Configuration

- Default config path: `${XDG_CONFIG_HOME:-~/.config}/wsl-backup/config.yaml`
- Optional config override: `BACKUP_CONFIG=/path/to/config.yaml`
- Starter config: [config.example.yaml](config.example.yaml)
- Rule file directory: `~/.config/wsl-backup/`
- Rule naming:
  - Include: `includes.<daily|weekly|monthly>.txt`
  - Exclude: `excludes.txt`
  - Cadence inheritance is cumulative:
    - `daily` uses `*.daily.txt`
    - `weekly` uses `*.daily.txt` + `*.weekly.txt`
    - `monthly` uses `*.daily.txt` + `*.weekly.txt` + `*.monthly.txt`
  - Include and exclude rules are authored from the WSL path perspective
  - Configure local repository paths in WSL form (for example, `/mnt/c/backups/repo`)
  - WSL runs filter include entries that start with `/mnt/<drive>/` before invoking `restic`
  - Windows runs translate local WSL paths, including rule entries and `--repo`, into `X:\...` before invoking `restic.exe`
  - Profile repositories are normalized and must be unique (for example, `/mnt/c/backups/repo` and `C:\\backups\\repo` are treated as the same target)

## Authentication

- Restic password is a hard requirement.
- Supported password sources (first available wins):
  - `RESTIC_PASSWORD` (direct value)
  - `WSL_BACKUP_PASSWORD_FILE` (preferred password file path)
  - `WSL_BACKUP_RESTIC_PASSWORD_FILE` (legacy alias)
  - `RESTIC_PASSWORD_FILE` (restic-compatible password file path)
  - systemd credentials directory (`$CREDENTIALS_DIRECTORY/restic_password`)
- If no valid password source is available, the command fails fast with an error.

## Usage

- This CLI is WSL-only; run it from a WSL shell (not from native Windows or a Dev Container).

- `wsl-backup run` performs a fast preflight check and fails fast on missing/mismatched restic versions.
- If preflight fails, run `wsl-backup setup` to install/upgrade `restic` across configured profiles.

### Typical flow

```sh
# One-time (or when versions drift)
wsl-backup setup

# Regular operation
wsl-backup run daily
wsl-backup run weekly
wsl-backup run monthly

# Restore examples
wsl-backup restore /tmp/restore-target
wsl-backup restore /tmp/restore-target --dry-run
```

### Edge cases

- `BACKUP_CONFIG` overrides config discovery and points directly to a config file.

```sh
BACKUP_CONFIG=/path/to/config.yaml wsl-backup setup
BACKUP_CONFIG=/path/to/config.yaml wsl-backup run daily

# Optional password file override
WSL_BACKUP_PASSWORD_FILE=/path/to/backup-password.txt \
BACKUP_CONFIG=/path/to/config.yaml \
wsl-backup run daily
```

- Missing rules behavior:
  - Missing include rule files fail fast for all inherited cadences.
  - Missing exclude rule file (`excludes.txt`) fails fast.

- `wsl-backup restore <target> --dry-run` passes `--dry-run` to `restic restore` and performs a non-writing preview restore.
- Additional restore args are forwarded after `--dry-run` (for example include/exclude filters).

- If installed through `wsl-sys-cli`, run the same commands as `sys wsl-backup ...`.

## Caveats

- `restic` stores symlinks as symlinks by default and does not follow them during backup. This behavior helps avoid recursive traversal from link loops. If symlink following is enabled explicitly in a restic invocation, traversal/loop risk must be evaluated separately.
- `use_fs_snapshot` is supported only for the `windows` profile.
- On the Windows profile, enabling `use_fs_snapshot` passes `--use-fs-snapshot` to `restic` and automatically launches `restic.exe` through an elevated UAC prompt (`Start-Process -Verb RunAs`). This uses Volume Shadow Copy Service (VSS) snapshots for each volume involved in the backup, so data is read from snapshots instead of the live filesystem and can include files locked by running processes.

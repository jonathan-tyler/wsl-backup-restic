# wsl-backup

Thin, predictable wrapper around `restic`, run from WSL and usable as a `wsl-sys-cli` extension.

## What it does

- Runs from WSL and targets cross-platform backup flows (WSL + Windows)
- Checks for matching `restic` versions on WSL (via `dnf`) and Windows (via `scoop`), and offers to install/upgrade when mismatched or missing
- Supports *cadences* of `daily`, `weekly`, and `monthly` for backup and reporting commands

## Configuration

- Default config path: `${XDG_CONFIG_HOME:-~/.config}/wsl-backup/config.yaml`
- Optional config override: `BACKUP_CONFIG=/path/to/config.yaml`
- Starter config: [config.example.yaml](config.example.yaml)
- Rule file directory: `~/.config/wsl-backup/`
- Rule naming: `<profile>.<include|exclude>.<daily|weekly|monthly>.txt`
  - Rules are checked for filesystem overlap

## Authentication

- Restic password is a hard requirement and is loaded from KeepassXC via `keepassxc-cli` for each restic command.
- Required config options:
  - `keepassxc_database` (path to `.kdbx` database)
  - `keepassxc_entry` (entry name containing Password)
- Optional environment overrides:
  - `WSL_BACKUP_KEEPASSXC_DATABASE`
  - `WSL_BACKUP_KEEPASSXC_ENTRY`
- If lookup fails (including locked database), the command fails fast with an error.

## Usage

- This CLI is WSL-only; run it from a WSL shell (not from native Windows or a Dev Container).

- `backup run` performs a fast preflight check and fails fast on missing/mismatched restic versions.
- If preflight fails, run `backup setup` to install/upgrade `restic` across configured profiles.

### Typical flow

```sh
# One-time (or when versions drift)
backup setup

# Regular operation
backup run daily
backup run weekly
backup run monthly

# Restore examples
backup restore /tmp/restore-target
backup restore /tmp/restore-target --dry-run
```

### Edge cases

- `BACKUP_CONFIG` overrides config discovery and points directly to a config file.

```sh
BACKUP_CONFIG=/path/to/config.yaml backup setup
BACKUP_CONFIG=/path/to/config.yaml backup run daily

# Optional KeepassXC overrides (config remains the default source)
WSL_BACKUP_KEEPASSXC_DATABASE=/path/to/vault.kdbx \
WSL_BACKUP_KEEPASSXC_ENTRY=restic/main \
BACKUP_CONFIG=/path/to/config.yaml \
backup run daily
```

- Missing rules behavior:
  - Missing include rule file (`<profile>.include.<cadence>.txt`) fails fast.
  - Missing exclude rule file (`<profile>.exclude.<cadence>.txt`) is allowed.
  - Overlap across profiles in include or exclude rules fails fast.

- `backup restore <target> --dry-run` passes `--dry-run` to `restic restore` and performs a non-writing preview restore.
- Additional restore args are forwarded after `--dry-run` (for example include/exclude filters).

- If installed through `wsl-sys-cli`, run the same commands as `sys backup ...`.

## Caveats

- `restic` stores symlinks as symlinks by default and does not follow them during backup. This behavior helps avoid recursive traversal from link loops. If symlink following is enabled explicitly in a restic invocation, traversal/loop risk must be evaluated separately.
- On Windows profiles, enabling `use_fs_snapshot` passes `--use-fs-snapshot` to `restic`, which uses Volume Shadow Copy Service (VSS) snapshots for each volume involved in the backup. Data is read from those snapshots instead of the live filesystem, which helps include files that are locked by other running processes.

# AGENTS.md

## Project Intent

- This project is a thin, predictable wrapper around `restic`.

## Rules

- `rustic` is a common typo for `restic`.  Assume the latter.
- Do not suggest adding new content to `AGENTS.md`, but you may add specific content when asked to.
- Use `wsl-backup` for user-facing command/config naming (for example CLI command text, config directory paths, and built binary output name).
- Use `wsl-backup-orchestrator` for repository/module naming.
- After significant non-documentation changes, or before handing work back to the user, rebuild the binary artifact with `go build -o out/wsl-backup ./cmd/backup`.
- Suggest backward-compatibility approaches when relevant, but do not implement backward-compatibility behavior unless explicitly requested.
- After completing the next set of code changes, run a build before finalizing your response; this does not apply when changes are documentation-only.
- Do not offer to run `build module` unless operating within the dev container.

# AGENTS.md

## Project Intent

- This project is a thin, predictable wrapper around `restic`.

## Rules

- Do not suggest adding new content to `AGENTS.md`, but you may add specific content when asked to.
- If a project name is needed for paths, executable names, or config paths, use `wsl-backup`.
- After significant changes, or before handing work back to the user, rebuild the binary artifact with `go build -o out/wsl-backup ./cmd/backup`.
- After completing the next set of code changes, run a build before finalizing your response.
- Do not offer to run `build module` unless operating within the dev container.

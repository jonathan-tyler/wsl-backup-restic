#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(CDPATH= cd -- "${script_dir}/.." && pwd)

container_engine=${CONTAINER_ENGINE:-podman}
image_tag=${WSL_BACKUP_BUILD_IMAGE:-localhost/wsl-backup-build:latest}
out_dir=${OUT_DIR:-${repo_dir}/out}
binary_name=${BINARY_NAME:-wsl-backup}

case "$(uname -m)" in
    x86_64)
        default_goarch=amd64
        ;;
    aarch64|arm64)
        default_goarch=arm64
        ;;
    armv7l)
        default_goarch=arm
        ;;
    *)
        default_goarch=$(uname -m)
        ;;
esac

goos=${GOOS:-linux}
goarch=${GOARCH:-${default_goarch}}
cgo_enabled=${CGO_ENABLED:-0}

if ! command -v "${container_engine}" >/dev/null 2>&1; then
    printf '%s\n' "error: required container engine not found: ${container_engine}" >&2
    exit 127
fi

mkdir -p "${out_dir}"

"${container_engine}" build \
    --file "${repo_dir}/build/Containerfile" \
    --tag "${image_tag}" \
    "${repo_dir}"

exec "${container_engine}" run --rm \
    --userns=keep-id \
    --user "$(id -u):$(id -g)" \
    --security-opt=no-new-privileges \
    -e GOOS="${goos}" \
    -e GOARCH="${goarch}" \
    -e CGO_ENABLED="${cgo_enabled}" \
    -v "${repo_dir}:/workspace:Z" \
    -v "${out_dir}:/out:Z" \
    -v wsl-backup-go-build-cache:/tmp/go-build:Z \
    -v wsl-backup-go-mod-cache:/tmp/go-mod:Z \
    -w /workspace \
    "${image_tag}" \
    sh -c "go build -trimpath -o /out/${binary_name} ./cmd/backup"
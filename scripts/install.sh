#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(CDPATH= cd -- "${script_dir}/.." && pwd)

binary_name=${BINARY_NAME:-wsl-backup}
out_dir=${OUT_DIR:-${repo_dir}/out}
prefix=${PREFIX:-${HOME}/.local}
bindir=${BINDIR:-${prefix}/bin}
source_binary=${out_dir}/${binary_name}
target_binary=${bindir}/${binary_name}
xdg_config_home=${XDG_CONFIG_HOME:-${HOME}/.config}
config_dir=${CONFIG_DIR:-${xdg_config_home}/wsl-backup}
config_example=${repo_dir}/config.example.yaml

ensure_empty_file() {
	path=$1

	if [ -e "${path}" ]; then
		return 0
	fi

	: >"${path}"
}

"${script_dir}/build-in-container.sh"

install -d "${bindir}"
install -m 0755 "${source_binary}" "${target_binary}"
install -d "${config_dir}"

if [ ! -e "${config_dir}/config.yaml" ]; then
	install -m 0644 "${config_example}" "${config_dir}/config.yaml"
fi

ensure_empty_file "${config_dir}/includes.daily.txt"
ensure_empty_file "${config_dir}/includes.weekly.txt"
ensure_empty_file "${config_dir}/includes.monthly.txt"
ensure_empty_file "${config_dir}/excludes.txt"

printf '%s\n' "installed ${binary_name} to ${target_binary}"
printf '%s\n' "ensured config scaffold in ${config_dir}"
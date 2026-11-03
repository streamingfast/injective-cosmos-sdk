#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
go_proxy_dir="${repo_root}/.modcache"
temp_go_proxy_dir="$(mktemp -d "${repo_root}/.modcache.tmp.XXXXXX")"
module_list_file="${temp_go_proxy_dir}/modules.txt"

cleanup() {
  rm -rf -- "${temp_go_proxy_dir}"
}
trap cleanup EXIT

(
  cd -- "${repo_root}"
  "${script_dir}/with-private-git.sh" go mod download
)

go_module_cache="$(go env GOMODCACHE)"
module_count=0
private_modules=()

(
  cd -- "${repo_root}"
  "${script_dir}/with-private-git.sh" go list -m \
    -f '{{if .Replace}}{{.Replace.Path}} {{.Replace.Version}}{{else}}{{.Path}} {{.Version}}{{end}}' all
) | LC_ALL=C sort -u > "${module_list_file}"

while read -r module_path module_version; do
  [[ -n "${module_path}" && -n "${module_version}" ]] || continue
  [[ "${module_path}" == github.com/InjectiveLabs/* ]] || continue

  private_modules+=("${module_path}@${module_version}")
done < "${module_list_file}"

if [[ ${#private_modules[@]} -eq 0 ]]; then
  echo "No InjectiveLabs modules were found for the Docker build." >&2
  exit 1
fi

# A restored Go cache may contain an extracted module or its go.mod without the
# corresponding proxy zip. Download every selected private module explicitly so
# the complete file-proxy artifact set is always available below.
(
  cd -- "${repo_root}"
  "${script_dir}/with-private-git.sh" go mod download "${private_modules[@]}"
)

while read -r module_path module_version; do
  [[ -n "${module_path}" && -n "${module_version}" ]] || continue
  [[ "${module_path}" == github.com/InjectiveLabs/* ]] || continue

  # Go escapes uppercase letters in module-cache paths as !<lowercase>.
  escaped_module_path="github.com/!injective!labs/${module_path#github.com/InjectiveLabs/}"
  source_version_dir="${go_module_cache}/cache/download/${escaped_module_path}/@v"
  target_version_dir="${temp_go_proxy_dir}/${escaped_module_path}/@v"
  mkdir -p -- "${target_version_dir}"

  for extension in info mod zip ziphash; do
    source_file="${source_version_dir}/${module_version}.${extension}"
    if [[ ! -f "${source_file}" ]]; then
      echo "Missing downloaded module artifact: ${source_file}" >&2
      exit 1
    fi
    cp -- "${source_file}" "${target_version_dir}/"
  done

  module_count=$((module_count + 1))
done < "${module_list_file}"

rm -- "${module_list_file}"

# This directory is fixed under the repository root and contains only generated,
# gitignored module-proxy data.
rm -rf -- "${go_proxy_dir}"
mv -- "${temp_go_proxy_dir}" "${go_proxy_dir}"
trap - EXIT

echo "Prepared ${module_count} InjectiveLabs module versions in ${go_proxy_dir}."

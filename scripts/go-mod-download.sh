#!/usr/bin/env bash
# Download Go modules through with-private-git.sh, so private InjectiveLabs
# modules resolve with GH_TOKEN. Pass module directories (relative to the
# repository root) to limit the download; with no arguments every module in
# the repository is downloaded.

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"

if [[ $# -gt 0 ]]; then
  modules=("$@")
else
  modules=()
  # A read loop instead of mapfile: macOS still ships bash 3.2.
  while IFS= read -r module; do
    modules+=("${module}")
  done < <(cd -- "${repo_root}" && git ls-files '*go.mod' | xargs -n1 dirname | LC_ALL=C sort)
fi

for module in "${modules[@]}"; do
  echo "Downloading modules for ${module}"
  "${script_dir}/with-private-git.sh" go -C "${repo_root}/${module}" mod download
done

#!/usr/bin/env bash
#
# Lint every Go module touched since the merge base with LINT_BASE (default
# origin/v0.50.x-inj), reporting only issues introduced since that merge base.
# Pre-existing findings in fork code don't fail unrelated changes. When the
# lint configuration or a Makefile changes, every module is linted.

set -e -o pipefail

REPO_ROOT="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )/.." &> /dev/null && pwd )"
LINT_BASE="${LINT_BASE:-origin/v0.50.x-inj}"

cd "${REPO_ROOT}"
merge_base="$(git merge-base "${LINT_BASE}" HEAD)"

# scripts/* modules only pin CI tools (behind the "tools" build tag) and have
# no packages to lint.
all_modules="$(git ls-files '*go.mod' | xargs -n1 dirname | grep -v '^scripts/' | LC_ALL=C sort)"
changed_files="$(git diff --name-only "${merge_base}" -- ; git ls-files --others --exclude-standard)"

if echo "${changed_files}" | grep -qE '(^|/)(\.golangci\.yml|Makefile)$'; then
  modules="${all_modules}"
else
  # Map each changed file to the innermost module that contains it.
  modules="$(
    echo "${changed_files}" | grep -E '\.go$|(^|/)go\.(mod|sum)$' | while IFS= read -r file; do
      dir="$(dirname "${file}")"
      while [[ "${dir}" != "." && ! -f "${dir}/go.mod" ]]; do
        dir="$(dirname "${dir}")"
      done
      echo "${dir}"
    done | grep -v '^scripts/' | LC_ALL=C sort -u
  )"
fi

if [[ -z "${modules}" ]]; then
  echo "no Go modules changed since ${LINT_BASE}"
  exit 0
fi

for module in ${modules}; do
  echo "linting ${module} [$(date -Iseconds -u)]"
  (cd "${REPO_ROOT}/${module}" &&
    golangci-lint run ./... -c "${REPO_ROOT}/.golangci.yml" \
      --new-from-merge-base="${merge_base}" "$@")
done

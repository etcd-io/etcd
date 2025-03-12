#!/usr/bin/env bash
# This script runs markdownlint-cli2 on changed files.
# Usage: ./markdown_lint.sh


# We source ./scripts/test_lib.sh, it sets the log functions and color variables.
source ./scripts/test_lib.sh

# When we source ./scripts/test_lib.sh, it has the line set -u which treats unset variables as errors.
# We need to unset the variable to avoid the error.
set +u -eo pipefail

if ! command markdownlint-cli2 dummy.md &>/dev/null; then
  log_error "markdownlint-cli2 needs to be installed."
  log_error "Please refer to https://github.com/DavidAnson/markdownlint-cli2?tab=readme-ov-file#install for installation instructions."
  exit 1
fi


if [ -z "${PULL_BASE_SHA}" ]; then
  echo "Empty base reference (\$PULL_BASE_SHA), assuming: main"
  PULL_BASE_SHA=main
fi

if [ -z "${PULL_PULL_SHA}" ]; then
  PULL_PULL_SHA="$(git rev-parse HEAD)"
  echo "Empty pull reference (\$PULL_PULL_SHA), assuming: ${PULL_PULL_SHA}"
fi

MD_LINT_URL_PREFIX="https://github.com/DavidAnson/markdownlint/blob/main/doc/"

mapfile -t changed_files < <(git diff "${PULL_BASE_SHA}" --name-only)
declare -A files_with_failures start_ranges end_ranges

for file in "${changed_files[@]}"; do
  if ! [[ "$file" =~ .md$ ]]; then
    continue
  fi

  # Find start and end ranges from changed files.
  start_ranges=()
  end_ranges=()
  # From https://github.com/paleite/eslint-plugin-diff/blob/46c5bcf296e9928db19333288457bf2805aad3b9/src/git.ts#L8-L27
  ranges=$(git diff "${PULL_BASE_SHA}" \
           --diff-algorithm=histogram \
           --diff-filter=ACM \
           --find-renames=100% \
           --no-ext-diff \
           --relative \
           --unified=0 -- "${file}" | \
    gawk 'match($0, /^@@\s-[0-9,]+\s\+([0-9]+)(,([0-9]+))?/, m) { \
               print m[1] ":" m[1] + ((m[3] == "") ? "0" : m[3]) }')
  i=0
  for range in ${ranges}; do
    start_ranges["${i}"]=$(echo "${range}" | awk -F: '{print $1}')
    end_ranges["${i}"]=$(echo "${range}" | awk -F: '{print $2}')
    i=$((1 + i))
  done
  if [ -z "${ranges}" ]; then
    start_ranges[0]=0
    end_ranges[0]=0
  fi

  i=0
  # Run markdownlint-cli2 with the changed file and print only the summary (stdout).
  markdownlint-cli2 "${file}" --config "${ETCD_ROOT_DIR}/tools/.markdownlint.jsonc" 2>/dev/null || true
  while IFS= read -r line; do
    line_number=$(echo "${line}" | awk -F: '{print $2}' | awk '{print $1}')
    while [ "${i}" -lt "${#end_ranges[@]}" ] && [ "${line_number}" -gt "${end_ranges["${i}"]}" ]; do
      i=$((1 + i))
    done
    rule=$(echo "${line}" | gawk 'match($2, /([^\/]+)/, m) {print tolower(m[1])}')
    lint_error="${line} (${MD_LINT_URL_PREFIX}${rule}.md)"

    if [ "${i}" -lt "${#start_ranges[@]}" ] && [ "${line_number}" -ge "${start_ranges["${i}"]}" ] && [ "${line_number}" -le "${end_ranges["${i}"]}" ]; then
      # Inside range with changes, raise an error.
      log_error "${lint_error}"
      files_with_failures["${file}"]=1
    else
      # Outside of range, raise a warning.
      log_warning "${lint_error}"
    fi
  done < <(markdownlint-cli2 "${file}" --config "${ETCD_ROOT_DIR}/tools/.markdownlint.jsonc" 2>&1 >/dev/null || true)
done

echo "Finished linting"

for file in "${!files_with_failures[@]}"; do
  log_error "${file} has linting issues"
done
if [ "${#files_with_failures[@]}" -gt "0" ]; then
  exit 1
fi


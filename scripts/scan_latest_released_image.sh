#!/usr/bin/env bash

set -euo pipefail

# Default image registry to use.
REGISTRY=${REGISTRY:-gcr.io/etcd-development/etcd}

# Default severity levels to report.
SEVERITY=${SEVERITY:-HIGH,CRITICAL}

source ./scripts/test_lib.sh

if ! command -v trivy >/dev/null; then
  log_error "Error: Cannot find trivy. Please follow the installation instructions at: https://trivy.dev/latest/getting-started/"
  exit 1
fi

# Returns the latest tag for the given branch.
function latest_branch_tag {
  local branch=$1
  local minor="${branch#release-}"

  git -c 'versionsort.suffix=-' \
    ls-remote --exit-code --refs --sort='version:refname' --tags origin "v${minor}"'*' \
    | tail --lines=1 \
    | cut --delimiter='/' --fields=3
}

function main {
  local current_branch
  current_branch=$(git rev-parse --abbrev-ref HEAD)
  if [[ ! "${current_branch}" =~ ^release-[0-9]+.[0-9]+$ ]]; then
    log_error "Error: This script is intended to be run only on stable release branches (current branch: ${current_branch})."
    return 1
  fi

  local latest_tag
  latest_tag=$(latest_branch_tag "$current_branch")

  trivy image --severity "${SEVERITY}" "${REGISTRY}:${latest_tag}"
}

main

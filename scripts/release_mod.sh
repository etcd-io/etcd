#!/usr/bin/env bash
# Copyright 2025 The etcd Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Examples:

# Edit go.mod files such that all etcd modules are pointing on given version:
#
# % DRY_RUN=false TARGET_VERSION="v3.5.13" ./scripts/release_mod.sh update_versions

# Tag latest commit with given version number for all the modules and push upstream:
#
# % DRY_RUN=false REMOTE_REPO="origin" RELEASE_VERSION="v3.5.13" ./scripts/release_mod.sh push_mod_tags

set -euo pipefail

source ./scripts/test_lib.sh

DRY_RUN=${DRY_RUN:-true}

# _cmd prints help message
function _cmd() {
  log_error "Command required: ${0} [cmd]"
  log_info "Available commands:"
  log_info "  - update_versions  - Updates all cross-module versions to \${TARGET_VERSION} in the local client."
  log_info "  - push_mod_tags    - Tags HEAD with \${RELEASE_VERSION} for all modules and pushes it to \${REMOTE_REPO}."
}

# update_module_version [v2version] [v3version]
#   Updates versions of cross-references in all internal references in current module.
function update_module_version() {
  local v3version="${1}"
  local v2version="${2}"
  local modules
  run go mod tidy
  modules=$(go mod edit -json | jq -r '.Require[] | select(.Indirect | not) | .Path')

  v3deps=$(echo "${modules}" | grep -E "${ROOT_MODULE}/.*/v3")
  for dep in ${v3deps}; do
    run go mod edit -require "${dep}@${v3version}"
  done

  v2deps=$(echo "${modules}" | grep -E "${ROOT_MODULE}/.*/v2")
  for dep in ${v2deps}; do
    run go mod edit -require "${dep}@${v2version}"
  done

  run go mod tidy
}

function mod_tidy_fix {
  run rm ./go.sum
  run go mod tidy || return 2
}

# Updates all cross-module versions to ${TARGET_VERSION} in local client.
function update_versions_cmd() {
  assert_no_git_modifications || return 2

  if [ -z "${TARGET_VERSION:-}" ]; then
    log_error "TARGET_VERSION environment variable not set. Set it to e.g. v3.5.10-alpha.0"
    return 2
  fi

  local v3version="${TARGET_VERSION}"
  local v2version
  # converts e.g. v3.5.0-alpha.0 --> v2.305.0-alpha.0
  # shellcheck disable=SC2001
  v2version="$(echo "${TARGET_VERSION}" | sed 's|^v3.\([0-9]*\).|v2.30\1.|g')"

  log_info "DRY_RUN       : ${DRY_RUN}"
  log_info "TARGET_VERSION: ${TARGET_VERSION}"
  log_info ""
  log_info "v3version: ${v3version}"
  log_info "v2version: ${v2version}"

  run_for_workspace_modules update_module_version "${v3version}" "${v2version}"
  run_for_workspace_modules mod_tidy_fix || exit 2
}

function get_gpg_key {
  gitemail=$(git config --get user.email)
  keyid=$(run gpg --list-keys --with-colons "${gitemail}" | awk -F: '/^pub:/ { print $5 }' | head -n 1)
  if [[ -z "${keyid}" ]]; then
    log_error "Failed to load gpg key. Is gpg set up correctly for etcd releases?"
    return 2
  fi
  echo "$keyid"
}

function push_mod_tags_cmd {
  assert_no_git_modifications || return 2

  if [ -z "${REMOTE_REPO:-}" ]; then
    log_error "REMOTE_REPO environment variable not set"
    return 2
  fi
  if [ -z "${RELEASE_VERSION:-}" ]; then
    log_error "RELEASE_VERSION environment variable not set. Set it to e.g. v3.5.13"
    return 2
  fi
  log_info "REMOTE_REPO:     ${REMOTE_REPO}"
  log_info "RELEASE_VERSION: ${RELEASE_VERSION}"

  local version="${RELEASE_VERSION}"
  local tags=()

  keyid=$(get_gpg_key) || return 2

  for module in $(modules); do
    # e.g. go.etcd.io/etcd/client/v3 --> client/v3, go.etcd.io/etcd/v3 --> v3
    local subdir="${module//${ROOT_MODULE}\//}"
    # strip the major version suffix, as it is not part of the tag path
    local prefix="${subdir%/v[23]}"
    local tag
    if [ "${prefix}" == "${subdir}" ]; then
      # the root module is tagged with the bare version
      tag="${version}"
    else
      tag="${prefix}/${version}"
    fi

    log_info "Tags for: ${module} version:${version} tag:${tag}"
    # The sleep is ugly hack that guarantees that 'git describe' will
    # consider main-module's tag as the latest.
    run sleep 2
    run git tag --local-user "${keyid}" --sign "${tag}" --message "${version}"
    tags+=("${tag}")
  done
  maybe_run git push -f "${REMOTE_REPO}" "${tags[@]}"
}

# only release_mod when called directly, not sourced
if echo "$0" | grep -E "release_mod.sh$" >/dev/null; then
  "${1}_cmd"

  if "${DRY_RUN}"; then
    log_info
    log_warning "WARNING: It was a DRY_RUN. No files were modified."
  fi
fi

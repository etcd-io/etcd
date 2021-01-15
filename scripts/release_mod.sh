#!/usr/bin/env bash

# Examples:

# Edit go.mod files such that all etcd modules are pointing on given version:
#
# % DRY_RUN=false TARGET_VERSION="v3.5.13" ./scripts/release_mod.sh update_versions

# Tag latest commit with current version number for all the modules and push upstream:
#
# % DRY_RUN=false REMOTE_REPO="origin" ./scripts/release_mod.sh push_mod_tags

set -e

DRY_RUN=${DRY_RUN:-true}

if ! [[ "$0" =~ scripts/release_mod.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

source ./scripts/test_lib.sh

# _cmd prints help message
function _cmd() {
  log_error "Command required: ${0} [cmd]"
  log_info "Available commands:"
  log_info "  - update_versions  - Updates all cross-module versions to \${TARGET_VERSION} in the local client."
  log_info "  - push_mod_tags    - Tags HEAD with all modules versions tags and pushes it to \${REMOTE_REPO}."
}

# maybe_run [cmd...] runs given command depending on the DRY_RUN flag.
function maybe_run() {
  if ${DRY_RUN}; then
    log_warning -e "# DRY_RUN:\n  % ${*}"
  else
    run "${@}"
  fi
}

# update_module_version [v2version] [v3version]
#   Updates versions of cross-references in all internal references in current module.
function update_module_version() {
  local v3version="${1}"
  local v2version="${2}"
  local modules
  modules=$(run go list -f '{{if not .Main}}{{if not .Indirect}}{{.Path}}{{end}}{{end}}' -m all)

  v3deps=$(echo "${modules}" | grep -E "${ROOT_MODULE}/.*/v3")
  for dep in ${v3deps}; do
    maybe_run go mod edit -require "${dep}@${v3version}"
  done

  v2deps=$(echo "${modules}" | grep -E "${ROOT_MODULE}/.*/v2")
  for dep in ${v2deps}; do
    maybe_run go mod edit -require "${dep}@${v2version}"
  done
}

# Updates all cross-module versions to ${TARGET_VERSION} in local client.
function update_versions_cmd() {
  assert_no_git_modifications || return 2

  if [ -z "${TARGET_VERSION}" ]; then
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

  run_for_modules update_module_version "${v3version}" "${v2version}"
}

function get_gpg_key {
  keyid=$(gpg --list-keys --with-colons| awk -F: '/^pub:/ { print $5 }')
  if [[ -z "${keyid}" ]]; then
    log_error "Failed to load gpg key. Is gpg set up correctly for etcd releases?"
    return 2
  fi
  echo "$keyid"
}

function push_mod_tags_cmd {
  assert_no_git_modifications || return 2

  if [ -z "${REMOTE_REPO}" ]; then
    log_error "REMOTE_REPO environment variable not set"
    return 2
  fi
  log_info "REMOTE_REPO:  ${REMOTE_REPO}"

  # Any module ccan be used for this
  local master_version
  master_version=$(go list -f '{{.Version}}' -m "${ROOT_MODULE}/api/v3")
  local tags=()

  keyid=$(get_gpg_key) || return 2

  for module in $(modules); do
    local version
    version=$(go list -f '{{.Version}}' -m "${module}")
    local path
    path=$(go list -f '{{.Path}}' -m "${module}")
    local subdir="${path//${ROOT_MODULE}\//}"
    local tag
    if [ -z "${version}" ]; then
      tag="${master_version}"
      version="${master_version}"
    else
      tag="${subdir///v[23]/}/${version}"
    fi

    log_info "Tags for: ${module} version:${version} tag:${tag}"
    # The sleep is ugly hack that guarantees that 'git describe' will
    # consider main-module's tag as the latest.
    run sleep 2
    maybe_run git tag --local-user "${keyid}" --sign "${tag}" --message "${version}"
    tags=("${tags[@]}" "${tag}")
  done
  maybe_run git push -f "${REMOTE_REPO}" "${tags[@]}"
}

"${1}_cmd"

if "${DRY_RUN}"; then
  log_info
  log_warning "WARNING: It was a DRY_RUN. No files were modified."
fi

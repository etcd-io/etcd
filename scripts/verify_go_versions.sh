#!/usr/bin/env bash

# This script verifies that the value of the toolchain directive in the
# go.mod files always match that of the .go-version file to ensure that
# we accidentally don't test and release with differing versions of Go.

set -euo pipefail

source ./scripts/test_lib.sh

target_go_version="${target_go_version:-"$(cat "${ETCD_ROOT_DIR}/.go-version")"}"
log_info "expected go toolchain directive: go${target_go_version}"
log_info

toolchain_out_of_sync="false"
go_line_violation="false"

# verify_go_versions takes a go.mod filepath as an argument
# and checks if:
#  (1) go directive <= version in .go-version
#  (2) toolchain directive == version in .go-version
function verify_go_versions() {
    # shellcheck disable=SC2086
    toolchain_version="$(go mod edit -json $1 | jq -r .Toolchain)"
    # shellcheck disable=SC2086
    go_line_version="$(go mod edit -json $1 | jq -r .Go)"
    if [[ "go${target_go_version}" != "${toolchain_version}" ]]; then
        log_error "go toolchain directive out of sync for $1, got: ${toolchain_version}"
        toolchain_out_of_sync="true"
    fi
    if ! printf '%s\n' "${go_line_version}" "${target_go_version}" | sort --check=silent --version-sort; then
        log_error "go directive in $1 is greater than maximum allowed: go${target_go_version}"
        go_line_violation="true"
    fi
}

while read -r mod; do
    verify_go_versions "${mod}";
done < <(find . -name 'go.mod')

if [[ "${toolchain_out_of_sync}" == "true" ]]; then
    log_error
    log_error "Please run scripts/sync_go_toolchain_directive.sh or update .go-version to rectify this error"
fi

if [[ "${go_line_violation}" == "true" ]]; then
    log_error
    log_error "Please update .go-version to rectify this error, any go directive should be <= .go-version"
fi

if [[ "${go_line_violation}" == "true" ]] || [[ "${toolchain_out_of_sync}" == "true" ]]; then
    exit 1
fi

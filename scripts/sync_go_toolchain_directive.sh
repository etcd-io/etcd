#!/usr/bin/env bash

# This script looks at the version present in the .go-version file and treats
# that to be the value of the toolchain directive that go should use. It then
# updates the toolchain directives of all go.mod files to reflect this version.
#
# We do this to ensure that .go-version acts as the source of truth for go versions.

set -euo pipefail

ROOT_MODULE="go.etcd.io/etcd"

if [[ "$(go list)" != "${ROOT_MODULE}" ]]; then
  echo "must be run from '${ROOT_MODULE}' module directory"
  exit 255
fi

ETCD_ROOT_DIR=$(go list -f '{{.Dir}}' "${ROOT_MODULE}")

TARGET_GO_VERSION="${TARGET_GO_VERSION:-"$(cat "${ETCD_ROOT_DIR}/.go-version")"}"
find . -name 'go.mod' -exec go mod edit -toolchain=go"${TARGET_GO_VERSION}" {} \;

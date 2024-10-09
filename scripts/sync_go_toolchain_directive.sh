#!/usr/bin/env bash

# This script looks at the version present in the .go-version file and treats
# that to be the value of the toolchain directive that go should use. It then
# updates the toolchain directives of all go.mod files to reflect this version.
#
# We do this to ensure that .go-version acts as the source of truth for go versions.

set -euo pipefail

source ./scripts/test_lib.sh

TARGET_GO_VERSION="${TARGET_GO_VERSION:-"$(cat "${ETCD_ROOT_DIR}/.go-version")"}"
find . -name 'go.mod' -exec go mod edit -toolchain=go"${TARGET_GO_VERSION}" {} \;

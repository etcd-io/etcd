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

# This script looks at the version present in the .go-version file and treats
# that to be the value of the toolchain directive that go should use. It then
# updates the toolchain directives of all go.mod files to reflect this version.
#
# We do this to ensure that .go-version acts as the source of truth for go versions.

set -euo pipefail

source ./scripts/test_lib.sh

TARGET_GO_VERSION="${TARGET_GO_VERSION:-"$(cat "${ETCD_ROOT_DIR}/.go-version")"}"
find . -name 'go.mod' -exec go mod edit -toolchain=go"${TARGET_GO_VERSION}" {} \;

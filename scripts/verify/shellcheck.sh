#!/usr/bin/env bash

# Copyright 2026 The etcd Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

ETCD_ROOT_DIR=${ETCD_ROOT_DIR:-$(git rev-parse --show-toplevel)}
source "${ETCD_ROOT_DIR}/scripts/test_lib.sh"
SHELLCHECK_VERSION=${SHELLCHECK_VERSION:-"v0.10.0"}

if [ -z "${OS:-}" ]; then
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
fi

if [ -z "${ARCH:-}" ]; then
  ARCH=$(uname -m)

  if [ "$ARCH" = "arm64" ]; then
    ARCH="aarch64"
  fi
fi

function generic_checker {
  local cmd=("$@")
  if ! output=$("${cmd[@]}"); then
    echo "${output}"
    log_error -e "FAIL: '${cmd[*]}' checking failed (!=0 return code)"
    return 255
  fi
  if [ -n "${output}" ]; then
    echo "${output}"
    log_error -e "FAIL: '${cmd[*]}' checking failed (printed output)"
    return 255
  fi
}

function main {
  SHELLCHECK=shellcheck
  if ! tool_exists "shellcheck" "https://github.com/koalaman/shellcheck#installing"; then
    log_callout "Installing shellcheck $SHELLCHECK_VERSION"
    wget -qO- "https://github.com/koalaman/shellcheck/releases/download/${SHELLCHECK_VERSION}/shellcheck-${SHELLCHECK_VERSION}.${OS}.${ARCH}.tar.xz" | tar -xJv -C /tmp/ --strip-components=1
    mkdir -p ./bin
    mv /tmp/shellcheck ./bin/
    SHELLCHECK=./bin/shellcheck
  fi
  generic_checker run ${SHELLCHECK} -fgcc scripts/*.sh
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main
fi

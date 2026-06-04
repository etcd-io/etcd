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

function main {
  local image

  log_callout "Testing devcontainer"
  image=$(awk -F\" 'match($2, /image/){print $4}' .devcontainer/devcontainer.json)
  if run docker run --rm \
    -v "${ETCD_ROOT_DIR}:/src" \
    -w /src \
    "${image}" \
    /bin/bash -c 'git config --global --add safe.directory /src; make build'; then
    log_success "SUCCESS: devcontainer tests ran succesfully"
  else
    log_error "FAIL: devcontainer failed to run"
    return 2
  fi
}

main

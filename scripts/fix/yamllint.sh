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

# Fixes linter issues in YAML files.

set -euo pipefail

ETCD_ROOT_DIR=${ETCD_ROOT_DIR:-$(git rev-parse --show-toplevel)}
source "${ETCD_ROOT_DIR}/scripts/test_lib.sh"

function main {
  run_go_tool github.com/google/yamlfmt/cmd/yamlfmt \
    -conf "${ETCD_ROOT_DIR}/tools/.yamlfmt" .
}

# only run when called directly, not sourced
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main
fi

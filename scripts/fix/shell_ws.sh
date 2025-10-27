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

# Fixes whitespaces in bash scripts.
set -euo pipefail

ETCD_ROOT_DIR=${ETCD_ROOT_DIR:-$(git rev-parse --show-toplevel)}
source "${ETCD_ROOT_DIR}/scripts/test_lib.sh"

function main {
  local TAB=$'\t'

  log_callout "Fixing whitespaces in bash scripts"
  # Makes sure all bash scripts do use '  ' (double space) for indention.
  log_cmd "find "${ETCD_ROOT_DIR}" -name '*.sh' -exec sed -i.bak 's|${TAB}|  |g' {} \;"
  find "${ETCD_ROOT_DIR}" -name '*.sh' -exec sed -i.bak "s|${TAB}|  |g" {} \;
  find "${ETCD_ROOT_DIR}" -name '*.sh.bak' -delete
}

# only run when called directly, not sourced
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main
fi

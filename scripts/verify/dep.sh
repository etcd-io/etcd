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

function dump_module_deps() {
  local json_mod
  json_mod=$(run go mod edit -json)

  local module
  if ! module=$(echo "${json_mod}" | jq -r .Module.Path); then
    return 255
  fi

  local require
  require=$(echo "${json_mod}" | jq -r '.Require')
  if [ "$require" == "null" ]; then
    return 0
  fi

  echo "$require" | jq -r '.[] | .Path+","+.Version+","+if .Indirect then " (indirect)" else "" end+",'"${module}"'"'
}

# Checks whether dependencies are consistent across modules
all_dependencies=$(run_for_workspace_modules dump_module_deps | sort) || exit 2

duplicates=$(echo "${all_dependencies}" | cut -d ',' -f 1,2 | sort | uniq | cut -d ',' -f 1 | sort | uniq -d) || exit 2

if [[ -n "${duplicates}" ]]; then
  for dup in ${duplicates}; do
    log_error "FAIL: inconsistent versions for dependency: ${dup}"
    echo "${all_dependencies}" | grep "${dup}," | sed 's|\([^,]*\),\([^,]*\),\([^,]*\),\([^,]*\)|  - \1@\2\3 from: \4|g'
  done

  log_error "FAIL: inconsistent dependencies"
  exit 2
fi

log_success "SUCCESS: dependencies are consistent across modules"

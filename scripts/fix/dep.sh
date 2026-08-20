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

# dump_module_deps outputs dependency records in CSV format:
#   <dep_path>,<version>,<direct|indirect>,<module_dir>
# Similar to scripts/test.sh but records the relative directory instead of
# the Go module path, so we can use it directly with run_for_module.
function dump_module_deps {
  local mod_dir
  mod_dir=$(realpath --relative-to="${ETCD_ROOT_DIR}" .)

  local json_mod
  json_mod=$(run go mod edit -json)

  local require
  require=$(echo "${json_mod}" | jq -r '.Require')
  if [ "$require" == "null" ]; then
    return 0
  fi

  echo "$require" | jq -r '.[] | .Path+","+.Version+","+if .Indirect then " (indirect)" else "" end+",'"${mod_dir}"'"'
}

function collect_all_deps {
  log_callout "Collecting dependencies from all modules..."
  run_for_workspace_modules dump_module_deps | sort
}

# find_inconsistencies outputs dependency paths that have version conflicts
# Input: collected dependency records (CSV output of collect_all_deps)
# Output: one dependency path per line for each inconsistent dependency
function find_inconsistencies {
  local all_deps="$1"
  echo "${all_deps}" | cut -d ',' -f 1,2 | sort | uniq | cut -d ',' -f 1 | sort | uniq -d
}

# select_latest_version picks the highest version of a dependency from
# the collected dependency records using sort -V (version sort).
# Arguments:
#   $1 - dependency path
#   $2 - all dependency records
# Output: the latest version string
function select_latest_version {
  local dep="$1"
  local all_deps="$2"
  echo "${all_deps}" | grep "^${dep}," | cut -d ',' -f 2 | sort -uV | tail -1
}

# fix_inconsistencies resolves dependency version conflicts by running
# `go get dep@latest_version` in each affected module, then `go mod tidy`.
# Arguments:
#   $1 - all dependency records
#   $2 - list of inconsistent dependency paths
# Returns non-zero on first failure
function fix_inconsistencies {
  local all_deps="$1"
  local inconsistent_deps="$2"

  for dup in ${inconsistent_deps}; do
    local latest
    latest=$(select_latest_version "${dup}" "${all_deps}")
    log_callout "Fixing ${dup} -> ${latest}"

    # Find which module dirs have this dep (at any version)
    local affected_dirs
    affected_dirs=$(echo "${all_deps}" | grep "^${dup}," | cut -d ',' -f 4 | sort -u)

    for mod_dir in ${affected_dirs}; do
      if ! run_for_module "${mod_dir}" run go get "${dup}@${latest}"; then
        log_error "FAIL: go get ${dup}@${latest} failed in module ${mod_dir}"
        return 1
      fi
    done
  done

  # After all go get operations, tidy each affected module.
  local all_affected_dirs
  all_affected_dirs=$(for dup in ${inconsistent_deps}; do
    echo "${all_deps}" | grep "^${dup}," | cut -d ',' -f 4 | sort -u
  done | sort -u)

  for mod_dir in ${all_affected_dirs}; do
    if ! run_for_module "${mod_dir}" run go mod tidy; then
      log_error "FAIL: go mod tidy failed in module ${mod_dir}"
      return 1
    fi
  done
}

function main {
  log_callout "Collecting all module dependencies..."
  local all_deps
  all_deps=$(collect_all_deps)
  if [[ -z "${all_deps}" ]]; then
    log_error "FAIL: dependency collection returned no results"
    exit 1
  fi

  local inconsistent_deps
  inconsistent_deps=$(find_inconsistencies "${all_deps}")
  if [[ -z "${inconsistent_deps}" ]]; then
    log_success "All dependencies are consistent"
    exit 0
  fi

  local inconsistent_count
  inconsistent_count=$(echo "${inconsistent_deps}" | wc -l)
  log_callout "Found ${inconsistent_count} inconsistent dependencies. Resolving..."

  if ! fix_inconsistencies "${all_deps}" "${inconsistent_deps}"; then
    log_error "FAIL: error while trying to fix inconsistencies"
    exit 1
  fi

  log_success "All dependency inconsistencies resolved"
}

main

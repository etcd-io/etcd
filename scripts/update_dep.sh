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
#
# Usage:
#    ./scripts/update_dep.sh module version
# or ./scripts/update_dep.sh module (to update to the latest version)
# e.g.
#   ./scripts/update_dep.sh github.com/golang/groupcache
#   ./scripts/update_dep.sh github.com/soheilhy/cmux v0.1.5
#
# Updates version of given dependency in all the modules that depend on the mod.

set -euo pipefail

source ./scripts/test_lib.sh

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    log_error "Illegal number of parameters. Usage: $0 module [version]"
    exit 1
fi

mod="$1"
ver="${2:-}"

function print_current_dep_version {
  log_info "${mod} version in all go.mod files:"
  find . -name go.mod -exec grep -H "^\s*${mod}\s" {} + | sed 's|:|\t|' || true
  printf "\n"
}

function is_fully_indirect {
  # Returns 0 (true) if the dependency is fully indirect, 1 (false) otherwise.
  local all_lines
  all_lines=$(find . -name go.mod -exec grep -E "^\s*${mod}\s" {} + || true)
  if [ -z "${all_lines}" ]; then
    # Not a dependency anywhere.
    return 0
  fi

  local direct_lines
  direct_lines=$(echo "${all_lines}" | grep -v "// indirect")

  if [ -z "${direct_lines}" ]; then
    return 0 # true, fully indirect
  fi
  return 1 # false, has direct dependencies
}

function update_module {
  # Check if the module is a dependency.
  if go list -m all | grep -q -E "^\s*${mod}\s"; then
    if [ -z "${ver}" ]; then
      log_info "Updating ${mod} to latest version in $(module_subdir)..."
      run go get -u "${mod}"
    else
      log_info "Updating ${mod} to version ${ver} in $(module_subdir)..."
      run go get "${mod}@${ver}"
    fi
  fi
}

print_current_dep_version
if is_fully_indirect; then
  log_warning "Dependency '${mod}' is fully indirect or not used. Nothing to do."
  exit 0
fi

run_for_modules update_module

./scripts/fix.sh
PASSES="dep" ./scripts/test.sh

print_current_dep_version

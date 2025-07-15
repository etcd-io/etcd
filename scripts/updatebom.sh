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

set -euo pipefail

source ./scripts/test_lib.sh

function bom_fixlet {
  log_callout "generating bill-of-materials.json"

  cp go.mod go.mod.tmp
  cp go.sum go.sum.tmp

  local modules
  # shellcheck disable=SC2207
  modules=($(modules_for_bom))

  # BOM file should be generated for linux. Otherwise running this command on other operating systems such as OSX
  # results in certain dependencies being excluded from the BOM file, such as procfs. 
  # For more info, https://github.com/etcd-io/etcd/issues/19665
  if GOOS=linux GOFLAGS=-mod=mod run_go_tool "github.com/appscodelabs/license-bill-of-materials" \
      --override-file ./bill-of-materials.override.json \
      "${modules[@]}" > ./bill-of-materials.json.tmp; then
    cp ./bill-of-materials.json.tmp ./bill-of-materials.json
    log_success "bom refreshed"
  else
    log_error "FAIL: bom refreshing failed"
    mv go.mod.tmp go.mod
    mv go.sum.tmp go.sum
    return 2
  fi
  mv go.mod.tmp go.mod
  mv go.sum.tmp go.sum
}

function bom_fix {
  # We regenerate bom from the tests directory, as it's a module
  # that depends on all other modules, so we can generate comprehensive content.
  # TODO: Migrate to root module, when root module depends on everything (including server & tests).
  run_for_module "." bom_fixlet
}

# only build when called directly, not sourced
if [[ "$0" =~ updatebom.sh$ ]]; then
  bom_fix
fi

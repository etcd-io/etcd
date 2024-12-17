#!/usr/bin/env bash

set -euo pipefail

source ./scripts/test_lib.sh

function bom_fixlet {
  log_callout "generating bill-of-materials.json"

  cp go.mod go.mod.tmp
  cp go.sum go.sum.tmp

  # bash 3.x compatible replacement of: mapfile -t modules < <(workspace_modules_without_tools)
  local modules=()
  while IFS= read -r line; do modules+=("$line"); done < <(workspace_relative_modules_without_tools)

  if run_go_tool "github.com/appscodelabs/license-bill-of-materials" \
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
  run bom_fixlet
}

# only build when called directly, not sourced
if [[ "$0" =~ updatebom.sh$ ]]; then
  bom_fix
fi

#!/usr/bin/env bash

set -e
source ./scripts/test_lib.sh

function bom_fixlet {
  log_callout "generating bill-of-materials.json"

  cp go.mod go.mod.tmp
  cp go.sum go.sum.tmp

  local modules
  # shellcheck disable=SC2207
  modules=($(modules_exp))

  if GOFLAGS=-mod=mod run_go_tool "github.com/coreos/license-bill-of-materials" \
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

#!/usr/bin/env bash

set -e

source ./scripts/test_lib.sh
source ./scripts/updatebom.sh

function mod_tidy_fix {
  run rm ./go.sum
  run go mod tidy || return 2
}

log_callout -e "\nFixing etcd code for you...\n"

run_for_modules run go fmt || exit 2
run_for_modules mod_tidy_fix || exit 2
run_for_module tests bom_fix || exit 2

log_success -e "\nSUCCESS: etcd code is fixed :)"

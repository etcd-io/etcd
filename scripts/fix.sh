#!/usr/bin/env bash

set -e

source ./scripts/test_lib.sh
source ./scripts/updatebom.sh

function mod_tidy_fix {
  run rm ./go.sum
  run go mod tidy || return 2
}

function bash_ws_fix {
  TAB=$'\t'

  log_callout "Fixing whitespaces in the bash scripts"
  # Makes sure all bash scripts do use '  ' (double space) for indention. 
  log_cmd "find ./ -name '*.sh' -print0 | xargs -0 sed -i.bak 's|${TAB}|  |g'"
  find ./ -name '*.sh' -print0 | xargs -0 sed -i.bak "s|${TAB}|  |g"
  find ./ -name '*.sh.bak' -print0 | xargs -0 rm
}

log_callout -e "\nFixing etcd code for you...\n"

run_for_modules run go fmt || exit 2
run_for_modules mod_tidy_fix || exit 2
run_for_module tests bom_fix || exit 2
bash_ws_fix || exit 2

log_success -e "\nSUCCESS: etcd code is fixed :)"

#!/usr/bin/env bash

set -euo pipefail

# Top level problems with modules can lead to test_lib being not functional
go mod tidy

source ./scripts/test_lib.sh
source ./scripts/updatebom.sh

# To fix according to newer version of go:
# go get golang.org/dl/gotip
# gotip download
# GO_CMD="gotip"
GO_CMD="go"

function mod_tidy_fix {
  local modules=("$@")
  for module in "${modules[@]}"; do
    local module_dir
    module_dir="${module%/...}"
    run_for_module "${module_dir}" run "${GO_CMD}" mod tidy
  done
}

function bash_ws_fix {
  TAB=$'\t'

  log_callout "Fixing whitespaces in the bash scripts"
  # Makes sure all bash scripts do use '  ' (double space) for indention. 
  log_cmd "find ./ -name '*.sh' -print0 | xargs -0 sed -i.bak 's|${TAB}|  |g'"
  find ./ -name '*.sh' -print0 | xargs -0 sed -i.bak "s|${TAB}|  |g"
  find ./ -name '*.sh.bak' -print0 | xargs -0 rm
}

log_callout -e "\\nFixing etcd code for you...\n"

run_for_all_modules mod_tidy_fix || exit 2
run run gofmt -w . || exit 2
bash_ws_fix || exit 2

log_success -e "\\nSUCCESS: etcd code is fixed :)"

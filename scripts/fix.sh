#!/usr/bin/env bash

set -e

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
  run rm ./go.sum
  run ${GO_CMD} mod tidy || return 2
}

function bash_ws_fix {
  TAB=$'\t'

  log_callout "Fixing whitespaces in the bash scripts"
  # Makes sure all bash scripts do use '  ' (double space) for indention. 
  log_cmd "find ./ -name '*.sh' -print0 | xargs -0 sed -i.bak 's|${TAB}|  |g'"
  find ./ -name '*.sh' -print0 | xargs -0 sed -i.bak "s|${TAB}|  |g"
  find ./ -name '*.sh.bak' -print0 | xargs -0 rm
}

log_callout -e "\\nFixing etcd code for you...\\n"

run_for_modules mod_tidy_fix || exit 2
run_for_modules run ${GO_CMD} fmt || exit 2
run_for_module tests bom_fix || exit 2
bash_ws_fix || exit 2

log_success -e "\\nSUCCESS: etcd code is fixed :)"

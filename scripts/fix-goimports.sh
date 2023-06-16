#!/usr/bin/env bash

set -euo pipefail

source ./scripts/test_lib.sh

ROOTDIR=$(pwd)

# To fix according to newer version of go:
# go get golang.org/dl/gotip
# gotip download
# GO_CMD="gotip"
GO_CMD="go"

ROOTDIR=$(pwd)

function go_imports_fix {
  GOFILES=$(run ${GO_CMD} list  --f "{{with \$d:=.}}{{range .GoFiles}}{{\$d.Dir}}/{{.}}{{\"\n\"}}{{end}}{{end}}" ./...)
  TESTGOFILES=$(run ${GO_CMD} list  --f "{{with \$d:=.}}{{range .TestGoFiles}}{{\$d.Dir}}/{{.}}{{\"\n\"}}{{end}}{{end}}" ./...)
  cd "${ROOTDIR}/tools/mod"
  echo "${GOFILES}" "${TESTGOFILES}" | grep -v '.gw.go' | grep -v '.pb.go' | xargs -n 100 go run golang.org/x/tools/cmd/goimports -w -local go.etcd.io
}

log_callout -e "\\nFixing goimports for you...\n"

run_for_modules go_imports_fix || exit 2

log_success -e "\\nSUCCESS: goimports are fixed :)"

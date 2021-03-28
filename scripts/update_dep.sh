#!/bin/bash 

# Usage:
#    ./scripts/update_dep.sh module version
# or ./scripts/update_dep.sh module
# e.g.
#   ./scripts/update_dep.sh github.com/golang/groupcache
#   ./scripts/update_dep.sh github.com/soheilhy/cmux v0.1.5
#
# Updates version of given dependency in all the modules that depend on the mod.

source ./scripts/test_lib.sh

mod="$1"
ver="$2"

function maybe_update_module {
  log_callout "Processing ${module}"
  run go mod tidy

  deps=$(go list -f '{{if not .Indirect}}{{if .Version}}{{.Path}},{{.Version}}{{end}}{{end}}' -m all)
  if [[ "$deps" == *"${mod}"* ]]; then
    if [ -z "${ver}" ]; then
      run go get "${mod}"
    else
      run go get "${mod}@${ver}"
    fi
  fi
 }

go mod tidy
run_for_modules maybe_update_module

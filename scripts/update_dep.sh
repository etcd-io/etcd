#!/bin/bash 

# Usage:
#    ./scripts/update_dep.sh module version
# or ./scripts/update_dep.sh module
# e.g.
#   ./scripts/update_dep.sh github.com/golang/groupcache
#   ./scripts/update_dep.sh github.com/soheilhy/cmux v0.1.5
#
# Updates version of given dependency in all the modules that depend on the mod.

set -euo pipefail

source ./scripts/test_lib.sh

if [ "$#" -ne 2 ]; then
    echo "Illegal number of parameters"
    exit 1
fi

mod="$1"
ver="$2"

function print_current_dep_version {
  echo "${mod} version in all go mod files"
  grep --exclude-dir=.git --include=\*.mod -Ri "^.*${mod} v.*$" | grep -v sum
  printf "\n\n"
}

function is_fully_indirect {
  # check if all lines end with "// indirect"
  # if grep found nothing, the error code will be non-zero
  ALL=$(grep --exclude-dir=.git --include=\*.mod -Ri "^.*${mod} v.*$" | grep -v sum | wc -l)
  ONLY_INDIRECT=$(grep --exclude-dir=.git --include=\*.mod -Ri "^.*${mod} v.*// indirect$" | grep -v sum | wc -l)
  if  [[ "$ALL" == "$ONLY_INDIRECT" ]]; then 
      echo "Fully indirect, we will terminate the script"
      exit 1
  else
      echo "Not fully indirect, we will perform dependency bump"
  fi
}

function update_module {
  run go mod tidy

  deps=$(go list -f '{{if .Version}}{{.Path}},{{.Version}}{{end}}' -m all)
  if [[ "$deps" == *"${mod}"* ]]; then
    if [ -z "${ver}" ]; then
      run go get -u "${mod}"
    else
      run go get "${mod}@${ver}"
    fi
  fi
}

print_current_dep_version
is_fully_indirect
run_for_modules update_module

./scripts/fix.sh
PASSES="dep" ./scripts/test.sh

print_current_dep_version

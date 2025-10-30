#!/bin/bash 
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
#
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

mod="$1"
ver="$2"

function maybe_update_module {
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

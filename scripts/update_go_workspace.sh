#!/usr/bin/env bash
#
# Copyright 2025 The etcd Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Based on k/k scripts/update-go-workspace.sh:
# https://github.com/kubernetes/kubernetes/blob/e2b96b25661849775dedf441b2f5c555392caa84/hack/update-go-workspace.sh

# This script generates go.work so that it includes all Go packages
# in this repo, with a few exceptions.

set -euo pipefail

source ./scripts/test_lib.sh

# Detect sed variant (BSD vs GNU) for in-place editing
# Source: https://stackoverflow.com/a/22084103 (CC BY-SA 4.0)
case "$OSTYPE" in
  darwin*|bsd*)
    sed_no_backup=( -i '' )
    ;;
  *)
    sed_no_backup=( -i )
    ;;
esac

# Avoid issues and remove the workspace files.
rm -f go.work go.work.sum

# Generate the workspace.
go work init
# Prepend comment header (portable sed syntax with literal newline after backslash)
sed "${sed_no_backup[@]}" '1i\
// This is a generated file. Do not edit directly.\

' go.work

# Include all submodules from the repository.
# Use while-read loop for portability (dirname -z is GNU-specific, not available on macOS)
git ls-files -z ':(glob)**/go.mod' | while IFS= read -r -d '' modfile; do
    go work edit -use "$(dirname "$modfile")"
done

go work edit -toolchain "go$(cat .go-version)"
go work edit -go "$(go mod edit -json | jq -r .Go)"

# generate go.work.sum
go mod download

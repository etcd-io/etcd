#!/usr/bin/env bash

# Copyright 2026 The etcd Authors
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

set -euo pipefail

ETCD_ROOT_DIR=${ETCD_ROOT_DIR:-$(git rev-parse --show-toplevel)}
source "${ETCD_ROOT_DIR}/scripts/test_lib.sh"

# `find .` is relative, so pin the working directory to the repo root.
cd "${ETCD_ROOT_DIR}"

log_callout "Ensuring no tab-based indentation in shell scripts"

if files=$(find . -name '*.sh' -print0 | xargs -0 grep -E -n $'^\s*\t'); then
  log_error "FAIL: found tab-based indentation in the following bash scripts. Use '  ' (double space):"
  log_error "${files}"
  log_warning "Suggestion: run \"make fix\" to address the issue."
  exit 255
fi

log_success "SUCCESS: no tabulators found."

#!/usr/bin/env bash
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

set -euo pipefail

source ./scripts/test_lib.sh
source ./scripts/updatebom.sh

function bash_ws_fix {
  TAB=$'\t'

  log_callout "Fixing whitespaces in the bash scripts"
  # Makes sure all bash scripts do use '  ' (double space) for indention. 
  log_cmd "find ./ -name '*.sh' -print0 | xargs -0 sed -i.bak 's|${TAB}|  |g'"
  find ./ -name '*.sh' -print0 | xargs -0 sed -i.bak "s|${TAB}|  |g"
  find ./ -name '*.sh.bak' -print0 | xargs -0 rm
}

log_callout -e "\\nFixing etcd code for you...\n"

bash_ws_fix || exit 2

log_success -e "\\nSUCCESS: etcd code is fixed :)"

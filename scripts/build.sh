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

# This scripts build the etcd binaries
# To build the tools, run `build_tools.sh`

set -euo pipefail

source ./scripts/test_lib.sh
source ./scripts/build_lib.sh

# only build when called directly, not sourced
if echo "$0" | grep -E "build(.sh)?$" >/dev/null; then
  run_build etcd_build
fi

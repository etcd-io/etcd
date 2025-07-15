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

GO_CMD="go"
fuzz_time=${FUZZ_TIME:-"300s"}
target_path=${TARGET_PATH:-"./server/etcdserver/api/v3rpc"}
TARGETS="FuzzTxnRangeRequest  FuzzTxnPutRequest  FuzzTxnDeleteRangeRequest"


for target in ${TARGETS}; do
    log_callout -e "\\nExecuting fuzzing with target ${target} in $target_path with a timeout of $fuzz_time\\n"
    run pushd "${target_path}"
        $GO_CMD test -fuzz "${target}" -fuzztime "${fuzz_time}"
    run popd
    log_success -e "\\COMPLETED: fuzzing with target $target in $target_path \\n"
done


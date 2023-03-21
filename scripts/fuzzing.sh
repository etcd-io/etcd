#!/usr/bin/env bash

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


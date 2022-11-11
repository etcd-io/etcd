#!/usr/bin/env bash
set -e
source ./scripts/test_lib.sh

GO_CMD="go"
fuzz_time=${FUZZ_TIME:-"300s"}

function run_fuzzer() {
    target_path=$1
    target=$2

    log_callout -e "\\nExecuting fuzzing with target ${target} in $target_path with a timeout of $fuzz_time\\n"
    run pushd "${target_path}"
        $GO_CMD test -fuzz "${target}" -fuzztime "${fuzz_time}"
    run popd
    log_success -e "\\COMPLETED: fuzzing with target $target in $target_path \\n"
}

# api fuzzers
target_path=${TARGET_PATH:-"./server/etcdserver/api/v3rpc"}
TARGETS="FuzzTxnRangeRequest  FuzzTxnPutRequest  FuzzTxnDeleteRangeRequest"


for target in ${TARGETS}; do
    run_fuzzer "${target_path}" "${target}"
done

# raft fuzzers
target_path=${TARGET_PATH:-"./raft"}
TARGETS="FuzzStep"

for target in ${TARGETS}; do
    run_fuzzer "${target_path}" "${target}"
done

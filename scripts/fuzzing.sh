#!/usr/bin/env bash
set -e
source ./scripts/test_lib.sh

GO_CMD="go"
fuzz_time=${FUZZ_TIME:-"300s"}
target_path=${TARGET_PATH:-"./server/etcdserver/api/v3rpc"}
fuzz_target=${FUZZ_TARGET:-"FuzzRangeRequest"}

log_callout -e "\\nExecuting fuzzing with target ${fuzz_target} in $target_path with a timeout of $fuzz_time\\n"
cd "$target_path"
$GO_CMD test -fuzz "$fuzz_target" -fuzztime "$fuzz_time"
log_success -e "\\COMPLETED: fuzzing with target $fuzz_target in $target_path \\n"

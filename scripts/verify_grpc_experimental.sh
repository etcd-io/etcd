#!/usr/bin/env bash

set -e

# Ensure we are at the root of the repo
ROOT_DIR=$(git rev-parse --show-toplevel)
cd "${ROOT_DIR}"

source ./scripts/test_lib.sh

TOOL_SRC="${ETCD_ROOT_DIR}/tools/check-grpc-experimental"
ALLOWLIST="${TOOL_SRC}/allowlist.txt"

FAILURES=0

for MOD_DIR in $(module_dirs); do
  echo "------------------------------------------------"
  echo "Checking module: ${MOD_DIR}"
  pushd "${MOD_DIR}" > /dev/null
    if ! go run "${TOOL_SRC}" -allow-list="${ALLOWLIST}" ./...; then
      echo "ERROR: Experimental usage found in ${MOD_DIR}"
      FAILURES=$((FAILURES+1))
    fi
  popd > /dev/null
done

echo "------------------------------------------------"
if [ "$FAILURES" -eq 0 ]; then
  echo "SUCCESS: No experimental gRPC APIs found in any module."
  exit 0
else
  echo "FAILURE: Found experimental gRPC API usage in ${FAILURES} module(s)."
  exit 1
fi
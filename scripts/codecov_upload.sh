#!/usr/bin/env bash

# Script used to collect and upload test coverage (mostly by travis).
# Usage ./test_coverage_upload.sh [log_file]

set -o pipefail

LOG_FILE=${1:-test-coverage.log}

# We collect the coverage
COVERDIR=covdir PASSES='build build_cov cov' ./test.sh 2>&1 | tee "${LOG_FILE}"
test_success="$?"

# We try to upload whatever we have:
bash <(curl -s https://codecov.io/bash) -f ./covdir/all.coverprofile -cF all || exit 2

# Expose the original status of the test coverage execution.
exit ${test_success}

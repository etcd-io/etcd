#!/usr/bin/env bash

# Script used to collect and upload test coverage (mostly by travis).
# Usage ./test_coverage_upload.sh [log_file]

set -o pipefail

# We try to upload whatever we have:
bash <(curl -s https://codecov.io/bash) -f ./covdir/all.coverprofile -cF all

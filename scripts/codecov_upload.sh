#!/usr/bin/env bash

# Script used to collect and upload test coverage.

set -o pipefail

# We try to upload whatever we have:
bash <(curl -s https://codecov.io/bash) -f "${COVERDIR}/all.coverprofile" \
  -cF all \
  -C "${PULL_PULL_SHA}" \
  -r "${REPO_OWNER}/${REPO_NAME}" \
  -P "${PULL_NUMBER}" \
  -b "${BUILD_ID}" \
  -B "${PULL_BASE_REF}" \
  -N "${PULL_BASE_SHA}"

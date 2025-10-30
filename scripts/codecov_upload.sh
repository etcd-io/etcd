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

# Script used to collect and upload test coverage.

set -o pipefail

# We try to upload whatever we have:
mkdir -p bin
curl -sf -o ./bin/codecov.sh https://codecov.io/bash

bash ./bin/codecov.sh -f "${COVERDIR}/all.coverprofile" \
  -cF all \
  -C "${PULL_PULL_SHA:-${PULL_BASE_SHA}}" \
  -r "${REPO_OWNER}/${REPO_NAME}" \
  -P "${PULL_NUMBER}" \
  -b "${BUILD_ID}" \
  -B "${PULL_BASE_REF}" \
  -N "${PULL_BASE_SHA}"

#!/usr/bin/env bash
# Copyright 2026 The etcd Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

ETCD_ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

BASE_IMAGE=$(sed -n 's/^FROM --platform=linux\/${ARCH} //p' "${ETCD_ROOT_DIR}/Dockerfile")
if [ -z "${BASE_IMAGE}" ]; then
  echo "error: Could not extract base image reference from Dockerfile" >&2
  exit 1
fi

if ! command -v trivy &>/dev/null; then
  echo "Installing trivy..." >&2
  go install github.com/aquasecurity/trivy/cmd/trivy@v0.70.0
  TRIVY="$(go env GOPATH)/bin/trivy"
else
  TRIVY=trivy
fi

FORMAT=${1:-table}
OUTPUT=${2:-}

ARGS=(--severity CRITICAL,HIGH --exit-code 1 --format "${FORMAT}")
if [ -n "${OUTPUT}" ]; then
  ARGS+=(--output "${OUTPUT}")
fi

exec "${TRIVY}" image "${ARGS[@]}" "${BASE_IMAGE}"

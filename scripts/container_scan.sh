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

LATEST_TAG=$(git -C "${ETCD_ROOT_DIR}" describe --tags --abbrev=0 --match 'v*')
if [ -z "${LATEST_TAG}" ]; then
  echo "error: Could not extract latest tag" >&2
  exit 1
fi
IMAGE="quay.io/coreos/etcd:${LATEST_TAG}"

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

exec "${TRIVY}" image "${ARGS[@]}" "${IMAGE}"

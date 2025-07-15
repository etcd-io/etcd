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
#
# Build all release binaries and images to directory ./release.
# Run from repository root.
#
set -euo pipefail

source ./scripts/test_lib.sh

VERSION=${1:-}
if [ -z "${VERSION}" ]; then
  echo "Usage: ${0} VERSION" >> /dev/stderr
  exit 255
fi

if ! command -v docker >/dev/null; then
    echo "cannot find docker"
    exit 1
fi

ETCD_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

pushd "${ETCD_ROOT}" >/dev/null
  log_callout "Building etcd binary..."
  ./scripts/build-binary.sh "${VERSION}"

  for TARGET_ARCH in "amd64" "arm64" "ppc64le" "s390x"; do
    log_callout "Building ${TARGET_ARCH} docker image..."
    GOOS=linux GOARCH=${TARGET_ARCH} BINARYDIR=release/etcd-${VERSION}-linux-${TARGET_ARCH} BUILDDIR=release ./scripts/build-docker.sh "${VERSION}"
  done
popd >/dev/null

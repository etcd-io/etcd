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
  VERSION=$(git describe --tags --always --dirty)
fi

if ! command -v docker >/dev/null; then
    echo "cannot find docker"
    exit 1
fi

if [ -n "${CI:-}" ]; then
  # there are few things missing in CI that we need to install
  # busybox tar doesn't support some of the flags we need for reproducible builds
  apk --no-cache add zip tar || true
fi

ETCD_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

VERSION=$(git describe --tags --always --dirty)

pushd "${ETCD_ROOT}" >/dev/null
  log_callout "Building etcd binary..."
  ./scripts/build-binary.sh "${VERSION}"

  BUILD_DIR=release ./scripts/build-docker.sh "${VERSION}"
  find release -name '*.*' -type f -maxdepth 1 -exec sha256sum {} \; | sed "s~release/~~" > release/SHA256SUMS
  if [ -n "${CI:-}" ]; then
    # cloudbuild will copy contents of this folder to GCS
    echo "Copying release artifacts to release/cloudbuild/${VERSION}"
    mkdir -p "release/cloudbuild/${VERSION}"
    cp release/SHA256SUMS release/cloudbuild/"${VERSION}"/SHA256SUMS
    cp release/*.zip release/cloudbuild/"${VERSION}"/ 
    cp release/*.tar.gz release/cloudbuild/"${VERSION}"/
    gcloud storage cp --recursive "release/cloudbuild" "gs://${GCS_LOCATION}"
  fi
popd >/dev/null

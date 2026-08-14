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
NO_DOCKER_PUSH=${2:-}

if ! command -v docker >/dev/null; then
    echo "cannot find docker"
    exit 1
fi

ETCD_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

pushd "${ETCD_ROOT}" >/dev/null
  log_callout "Building etcd binary..."
  ./scripts/build-binary.sh "${VERSION}"

  log_callout "Building Docker images..."
  OCI_REGISTRY="${OCI_REGISTRY:-}" OCI_PATH="${OCI_PATH:-}" NO_DOCKER_PUSH="${NO_DOCKER_PUSH}" BUILD_DIR=release ./scripts/build-docker.sh "${VERSION}"

  find release -name '*.*' -type f -maxdepth 1 -exec sha256sum {} \; | sed "s~release/~~" > release/SHA256SUMS
  if [ -n "${PUBLISH_TO_GCS:-}" ]; then
    # cloudbuild will copy contents of this folder to GCS
    echo "Copying release artifacts to release/cloudbuild/${VERSION}"
    mkdir -p "release/cloudbuild/${VERSION}"
    cp release/SHA256SUMS release/cloudbuild/"${VERSION}"/SHA256SUMS
    cp release/*.zip release/cloudbuild/"${VERSION}"/ 
    cp release/*.tar.gz release/cloudbuild/"${VERSION}"/
    gcloud storage cp --recursive "release/cloudbuild/" "gs://${GCS_LOCATION}"

    if [[ "${VERSION}" =~ ^v[0-9]+.[0-9]+.[0-9]+(-[a-zA-Z]+.[0-9]+)?$ ]]; then
      echo "Updating latest release artifacts in gs://${GCS_LOCATION}/latest"
      gcloud storage cp --recursive "gs://${GCS_LOCATION}/${VERSION}/" "gs://${GCS_LOCATION}/latest/"
    fi

  fi
popd >/dev/null

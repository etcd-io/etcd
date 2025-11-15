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

set -euo pipefail

source ./scripts/test_lib.sh

VERSION=${1:-}
if [ -z "$VERSION" ]; then
  echo "Usage: ${0} VERSION" >&2
  exit 1
fi

PLATFORMS=${2:-"linux/amd64,linux/arm64,linux/ppc64le,linux/s390x"}

if [ -n "${CI:-}" ]; then
  docker run --privileged --rm tonistiigi/binfmt --install all
  docker buildx create \
    --name multiarch-multiplatform-builder \
    --driver docker-container \
    --bootstrap --use
fi

if [ -z "${REGISTRY:-}" ]; then
    docker build --build-arg="VERSION=${VERSION}" \
      --build-arg="BUILD_DIR=${BUILD_DIR}" \
      --platform="${PLATFORMS}" \
      -t "gcr.io/etcd-development/etcd:${VERSION}" \
      -t "quay.io/coreos/etcd:${VERSION}" \
      .
    # we should deprecate publishing tags for specific architectures and use the multi-arch image
    for arch in amd64 arm64 ppc64le s390x; do
        log_callout "Building ${arch} docker image..."
        docker build --build-arg="VERSION=${VERSION}" \
          --build-arg="BUILD_DIR=${BUILD_DIR}" \
          --platform="linux/${arch}" \
          -t "gcr.io/etcd-development/etcd:${VERSION}-${arch}" \
          -t "quay.io/coreos/etcd:${VERSION}-${arch}" \
          .
    done

else
    docker buildx build --build-arg="VERSION=${VERSION}" \
      --build-arg="BUILD_DIR=${BUILD_DIR}" \
      --platform="${PLATFORMS}" \
      -t "${REGISTRY}/etcd:${VERSION}" \
      --push \
      .
    # we should deprecate publishing tags for specific architectures and use the multi-arch image
    for arch in amd64 arm64 ppc64le s390x; do
        log_callout "Building ${arch} docker image..."
        docker buildx build --build-arg="VERSION=${VERSION}" \
          --build-arg="BUILD_DIR=${BUILD_DIR}" \
          --platform="linux/${arch}" \
          -t "${REGISTRY}/etcd:${VERSION}-${arch}" \
          --push \
          .
    done
fi

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

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 VERSION" >&2
  exit 1
fi

VERSION=${1}
if [ -z "$VERSION" ]; then
  echo "Usage: ${0} VERSION" >&2
  exit 1
fi

BUILD_DIR=${BUILD_DIR:-release}
mkdir -p "${BUILD_DIR}"

PLATFORMS=${PLATFORMS:-"linux/amd64,linux/arm64,linux/ppc64le,linux/s390x"}

for platform in $(echo "${PLATFORMS}" | tr ',' ' '); do
  RELEASE="etcd-${VERSION}-linux-${platform#linux/}"
  if [ ! -d "${BUILD_DIR}/${RELEASE}" ]; then
    TARFILE="${RELEASE}.tar.gz"
    TARURL="https://github.com/etcd-io/etcd/releases/download/${VERSION}/${TARFILE}"
    if ! curl -f -L -o "${BUILD_DIR}/${TARFILE}" "${TARURL}" ; then
      echo "Failed to download ${TARURL}."
      exit 1
    fi
    tar -C "${BUILD_DIR}" -zvxf "${BUILD_DIR}/${TARFILE}"
  fi
done

tag_args=()
if [ -z "${REGISTRY:-}" ]; then
  tag_args+=("-t" "gcr.io/etcd-development/etcd:${VERSION}")
  tag_args+=("-t" "quay.io/coreos/etcd:${VERSION}")
else
  tag_args+=("-t" "${REGISTRY}/etcd:${VERSION}")
fi

if [ -n "${CI:-}" ]; then
  docker run --privileged --rm tonistiigi/binfmt --install all
  docker buildx create \
    --name multiarch-multiplatform-builder \
    --driver docker-container \
    --bootstrap --use
fi

docker buildx build --build-arg="VERSION=${VERSION}" \
  --build-arg="BUILD_DIR=${BUILD_DIR}" \
  --platform="${PLATFORMS}" \
  --load \
  "${tag_args[@]}" \
  .

for platform in $(echo "${PLATFORMS}" | tr ',' ' '); do
  platform_tag_args=()
  for arg in "${tag_args[@]}"; do
    if [ "${arg}" != "-t" ]; then
      arg+="-${platform#linux/}"
    fi
    platform_tag_args+=("$arg")
  done

  docker buildx build --build-arg="VERSION=${VERSION}" \
    --build-arg="BUILD_DIR=${BUILD_DIR}" \
    --platform="${platform}" \
    --load \
    "${platform_tag_args[@]}" \
    .
done

#!/usr/bin/env bash
#
# Build all release binaries and images to directory ./release.
# Run from repository root.
#
set -e

source ./scripts/test_lib.sh

VERSION=$1
if [ -z "${VERSION}" ]; then
  echo "Usage: ${0} VERSION" >> /dev/stderr
  exit 255
fi

if ! command -v docker >/dev/null; then
    echo "cannot find docker"
    exit 1
fi

function package {
  local target=${1}
  local srcdir="${2}/bin"

  local ccdir="${srcdir}/${GOOS}_${GOARCH}"
  if [ -d "${ccdir}" ]; then
    srcdir="${ccdir}"
  fi
  local ext=""
  if [ "${GOOS}" == "windows" ]; then
    ext=".exe"
  fi
  for bin in etcd etcdctl etcdutl; do
    cp "${srcdir}/${bin}" "${target}/${bin}${ext}"
  done

  cp etcd/README.md "${target}"/README.md
  cp etcd/etcdctl/README.md "${target}"/README-etcdctl.md
  cp etcd/etcdctl/READMEv2.md "${target}"/READMEv2-etcdctl.md
  cp etcd/etcdutl/README.md "${target}"/README-etcdutl.md

  cp -R etcd/Documentation "${target}"/Documentation
}

ETCD_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

pushd "${ETCD_ROOT}" >/dev/null
  for GOOS in darwin windows linux; do
    TARGET_ARCHS=("amd64")

    if [ ${GOOS} == "linux" ]; then
      TARGET_ARCHS+=("arm64")
      TARGET_ARCHS+=("ppc64le")
      TARGET_ARCHS+=("s390x")
    fi

    if [ ${GOOS} == "darwin" ]; then
      TARGET_ARCHS+=("arm64")
    fi

    for GOARCH in "${TARGET_ARCHS[@]}"; do
      IMAGE="gcr.io/etcd-development/etcd:${VERSION}-${GOOS}-${GOARCH}"
      if [ "${GOOS}" == "linux" ]; then
        IMAGE="gcr.io/etcd-development/etcd:${VERSION}-${GOARCH}"
      fi

      docker build -t "${IMAGE}" --build-arg GOARCH=${GOARCH} --build-arg GOOS=${GOOS} .
    done
  done
popd >/dev/null

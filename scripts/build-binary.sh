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

VER=${1:-}

if [ -z "$VER" ]; then
  VER=$(git describe --tags --always --dirty)
fi


function package {
  local target=${1}

  cp README.md "${target}"/README.md
  cp etcdctl/README.md "${target}"/README-etcdctl.md
  cp etcdctl/READMEv2.md "${target}"/READMEv2-etcdctl.md
  cp etcdutl/README.md "${target}"/README-etcdutl.md
  cp -R Documentation "${target}/"
}

function main {
  mkdir -p release
  local tarcmd=tar
  if [ "$(go env GOOS)" == "darwin" ]; then
    tarcmd=gtar
  fi
  for os in darwin windows linux; do
    export GOOS=${os}
    TARGET_ARCHS=("amd64")

    if [ ${GOOS} == "linux" ]; then
      TARGET_ARCHS+=("arm64")
      TARGET_ARCHS+=("ppc64le")
      TARGET_ARCHS+=("s390x")
    fi

    if [ ${GOOS} == "darwin" ]; then
      TARGET_ARCHS+=("arm64")
    fi

    for TARGET_ARCH in "${TARGET_ARCHS[@]}"; do
      export GOARCH=${TARGET_ARCH}

      TARGET="release/etcd-${VER}-${GOOS}-${GOARCH}"
      VERSION="${VER}" BINDIR="${TARGET}" GO_LDFLAGS="-s -w" scripts/build.sh
      package "${TARGET}"

      if [ ${GOOS} == "linux" ]; then
        # https://reproducible-builds.org/docs/archives/#full-example
        GZIP=-n ${tarcmd} --sort=name \
          --mtime="UTC 1970-01-01" \
          --owner=0 --group=0 --numeric-owner \
          --pax-option=exthdr.name=%d/PaxHeaders/%f,delete=atime,delete=ctime \
          -zcf "${TARGET}.tar.gz" "${TARGET}"
        echo "Wrote ${TARGET}.tar.gz"
      else
        zip -qrX "${TARGET}.zip" "${TARGET}"
        echo "Wrote ${TARGET}.zip"
      fi
    done
  done
}

main

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

ETCD_ROOT_DIR=${ETCD_ROOT_DIR:-$(git rev-parse --show-toplevel)}
source "${ETCD_ROOT_DIR}/scripts/test_lib.sh"

MARKDOWN_MARKER_VERSION=${MARKDOWN_MARKER_VERSION:="v0.10.0"}

if [ -z "${OS:-}" ]; then
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
fi

if [ -z "${ARCH:-}" ]; then
  ARCH=$(uname -m)

  if [ "$ARCH" = "arm64" ]; then
    ARCH="aarch64"
  fi
fi

marker="marker"
# TODO: check other markdown files when marker handles headers with '[]'
if ! tool_exists "$marker" "https://crates.io/crates/marker"; then
  log_callout "Installing markdown marker $MARKDOWN_MARKER_VERSION"
  MARKER_OS=$OS
  if [ "$OS" = "darwin" ]; then
    MARKER_OS="apple-darwin"
  elif [ "$OS" = "linux" ]; then
    MARKER_OS="unknown-linux-musl"
  fi

  wget -qO- "https://github.com/crawford/marker/releases/download/${MARKDOWN_MARKER_VERSION}/marker-${MARKDOWN_MARKER_VERSION}-${ARCH}-${MARKER_OS}.tar.gz" | tar -xzv -C /tmp/ --strip-components=1 >/dev/null
  mkdir -p ./bin
  mv /tmp/marker ./bin/
  marker=./bin/marker
fi

generic_checker run "${marker}" --skip-http --allow-absolute-paths --root "${ETCD_ROOT_DIR}" -e ./CHANGELOG -e ./etcdctl -e etcdutl -e ./tools 2>&1

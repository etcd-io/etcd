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

function install_golangci_lint() {
    echo "Installing golangci-lint ${GOLANGCI_LINT_VERSION}"
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "${GOPATH}/bin" "${GOLANGCI_LINT_VERSION}"
}

GOLANGCI_LINT_VERSION=$(cd tools/mod && go list -m -f '{{.Version}}' github.com/golangci/golangci-lint)
echo "golangci-lint version: $GOLANGCI_LINT_VERSION"
GOLANGCI_LINT_PRESENT=$(which golangci-lint)

if [ -z "$GOLANGCI_LINT_PRESENT" ]; then
    echo "golangci-lint is not available"
    install_golangci_lint
    exit 0
fi
GOLANGCI_LINT_INSTALLED=v$(golangci-lint version | grep -oP 'version \K[0-9.]+')

if [ "$GOLANGCI_LINT_VERSION" != "$GOLANGCI_LINT_INSTALLED" ]; then
    echo "different golangci-lint version installed: $GOLANGCI_LINT_INSTALLED"
    install_golangci_lint
    echo "golangci-lint version: $GOLANGCI_LINT_VERSION"
fi

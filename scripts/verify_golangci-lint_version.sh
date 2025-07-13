#!/bin/bash
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

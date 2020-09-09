#!/usr/bin/env bash

# Usage ./install_tool.sh {go_module}
#
# Install given tool and makes it available on $PATH (assuming standard config),
# without modification to vendor or go.mod file.
#
# When https://github.com/golang/go/issues/40276 is implemented, usage
# of this script should get replaced by pure: 
# 
# go install {go_module}@latest
# 
set -e

>&2 echo "installing: ${1}"
(
  cd "$(mktemp -d)"
  GO111MODULE=on go get "$1"
)

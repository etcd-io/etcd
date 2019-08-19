#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ scripts/updatedep.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# NOTE: just run
rm -rf ./vendor
glide install --strip-vendor --skip-test .
glide vc --only-code --no-tests

# ref. https://github.com/kubernetes/kubernetes/pull/81434
exit 0




if [ -d "gopath.proto" ]; then
  # gopath.proto is created by genproto.sh and it thoroughly messes
  # with go mod.
  echo "Remove gopath.proto before running this script"
  exit 255
fi

if [[ $(go version) != "go version go1.12"* ]]; then
  echo "expect Go 1.12+, got:" "$(go version)"
  exit 255
fi

GO111MODULE=on go mod tidy -v
GO111MODULE=on go mod vendor -v

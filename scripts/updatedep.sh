#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ scripts/updatedep.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

if [[ $(go version) != "go version go1.12"* ]]; then
  echo "expect Go 1.12+, got:" "$(go version)"
  exit 255
fi

GO111MODULE=on go mod tidy -v
GO111MODULE=on go mod vendor -v

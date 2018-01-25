#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ scripts/updatedep.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

go get -v -u github.com/golang/dep/cmd/dep
dep ensure -v
dep prune

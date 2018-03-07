#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ scripts/updatedep.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

go get -v -u github.com/golang/dep/cmd/dep

if [[ -z "$1" ]]; then
  echo "dep ensure on all packages"
  dep ensure -v
else
  echo "dep update on" "$1"
  # shellcheck disable=SC2086
  dep ensure -v -update $1
fi

dep prune

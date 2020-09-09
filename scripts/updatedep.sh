#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ scripts/updatedep.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

if [ -d "gopath.proto" ]; then
  # gopath.proto is created by genproto.sh and it thoroughly messes
  # with go mod.
  echo "Remove gopath.proto before running this script"
  exit 255
fi

if [[ $(go version) < "go version go1.14" ]]; then
  echo "expect Go 1.14+, got:" "$(go version)"
  exit 255
fi

GO111MODULE=on go mod tidy -v
GO111MODULE=on go mod vendor -v

RED='\033[0;31m'
NC='\033[0m' # No Color
echo -e "\n${RED} WARNING: In etcd >=3.5 we use go modules rather than vendoring" 
echo -e "${RED}          Please refactor your logic to depend on modules directly.${NC}"


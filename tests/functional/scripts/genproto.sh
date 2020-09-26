#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ "scripts/genproto.sh" ]]; then
  echo "must be run from repository root"
  exit 255
fi

# for now, be conservative about what version of protoc we expect
if ! [[ $(protoc --version) =~ "3.7.1" ]]; then
  echo "could not find protoc 3.7.1, is it installed + in PATH?"
  exit 255
fi

echo "Installing gogo/protobuf..."
GOGOPROTO_ROOT="$GOPATH/src/github.com/gogo/protobuf"
rm -rf $GOGOPROTO_ROOT
go get -v github.com/gogo/protobuf/{proto,protoc-gen-gogo,gogoproto,protoc-gen-gofast}
go get -v golang.org/x/tools/cmd/goimports
pushd "${GOGOPROTO_ROOT}"
  git reset --hard HEAD
  make install
popd

printf "Generating agent\n"
protoc --gofast_out=plugins=grpc:. \
  --proto_path=$GOPATH/src:$GOPATH/src/github.com/gogo/protobuf/protobuf:. \
  rpcpb/*.proto;

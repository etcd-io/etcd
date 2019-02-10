#!/usr/bin/env bash

set -e

if ! [[ "$0" =~ scripts/update-clientv2.sh ]]; then
	echo "must be run from repository root"
	exit 255
fi

echo "installing 'client/keys.generated.go'"
go get -v -u github.com/ugorji/go/codec/codecgen

echo "generating client/keys.generated.go"
pushd client
codecgen -o ./keys.generated.go ./keys.go
popd

echo "generated client/keys.generated.go"

#!/bin/sh
set -e

if [ -z "$PKG" ]; then
    PKG="./store ./server ./server/v2/tests ./mod/lock/v2/tests"
fi

# Get GOPATH, etc from build
. ./build

# use right GOPATH
export GOPATH="${PWD}"

# Unit tests
for i in $PKG
do
    go test -i $i
    go test -v $i
done

# Functional tests
go test -i ./tests/functional
ETCD_BIN_PATH=$(pwd)/etcd go test -v ./tests/functional

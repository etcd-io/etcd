#!/bin/sh
set -e

PKGS="./mod/lock/tests"
# PKGS="./store ./server ./server/v2/tests"

# Get GOPATH, etc from build
. ./build

# use right GOPATH
export GOPATH="${PWD}"

# Unit tests
for PKG in $PKGS
do
    go test -i $PKG
    go test -v $PKG
done

# Functional tests
go test -i ./tests/functional
ETCD_BIN_PATH=$(pwd)/etcd go test -v ./tests/functional

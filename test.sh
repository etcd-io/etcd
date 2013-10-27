#!/bin/sh
set -e

# Get GOPATH, etc from build
. ./build

# use right GOPATH
export GOPATH="${PWD}"

# Unit tests
go test -i ./server
go test -v ./server

go test -i ./server/v2/tests
go test -v ./server/v2/tests

go test -i ./store
go test -v ./store

# Functional tests
go test -i ./tests/functional
ETCD_BIN_PATH=$(pwd)/etcd go test -v ./tests/functional

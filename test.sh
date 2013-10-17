#!/bin/sh
set -e

# Get GOPATH, etc from build
. ./build

# Unit tests
go test -v ./store

# Functional tests
ETCD_BIN_PATH=$(pwd)/etcd go test -v ./tests/functional

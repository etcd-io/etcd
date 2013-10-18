#!/bin/sh
set -e

# Unit tests
echo "-- UNIT TESTS --"
go test -v ./server/v2/tests
go test -v ./store

# Get GOPATH, etc from build
echo "-- BUILDING BINARY --"
. ./build

# Functional tests
echo "-- FUNCTIONAL TESTS --"
ETCD_BIN_PATH=$(PWD)/etcd go test -v ./tests/functional

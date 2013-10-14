#!/bin/sh

# Get GOPATH, etc from build
. ./build

# Run the tests!
go test -i
go test -v

# Run the functional tests!
go test -i github.com/coreos/etcd-test-runner
ETCD_BIN_PATH=$(pwd)/etcd go test -v github.com/coreos/etcd-test-runner

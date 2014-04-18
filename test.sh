#!/bin/sh -e

. ./build

go test -i ./etcd
go test -v ./etcd -race

go test -i ./http
go test -v ./http -race

go test -i ./store
go test -v ./store -race

go test -i ./server
go test -v ./server -race

go test -i ./config
go test -v ./config -race

go test -i ./server/v1/tests
go test -v ./server/v1/tests -race

go test -i ./server/v2/tests
go test -v ./server/v2/tests -race

# Mod is deprecated temporarily.
# go test -i ./mod/lock/v2/tests
# go test -v ./mod/lock/v2/tests

go test -i ./pkg/btrfs
go test -v ./pkg/btrfs

go test -i ./tests/functional
ETCD_BIN_PATH=$(pwd)/bin/etcd go test -v ./tests/functional -race

fmtRes=`gofmt -l $GOFMTPATH`
if [ "$fmtRes" != "" ]; then
	echo "Failed to pass golang format checking."
	echo "Please gofmt modified go files, or run './build --fmt'."
	exit 1
fi

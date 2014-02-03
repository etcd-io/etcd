#!/bin/sh -e

source ./build

go test -i ./store
go test -v ./store

go test -i ./server
go test -v ./server

go test -i ./server/v2/tests
go test -v ./server/v2/tests

go test -i ./mod/lock/v2/tests
go test -v ./mod/lock/v2/tests

go test -i ./tests/functional
ETCD_BIN_PATH=$(pwd)/bin/etcd go test -v ./tests/functional

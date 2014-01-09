#!/bin/sh -e
go run third_party.go test -i ./store
go run third_party.go test -v ./store

go run third_party.go test -i ./server
go run third_party.go test -v ./server

go run third_party.go test -i ./server/v2/tests
go run third_party.go test -v ./server/v2/tests

go run third_party.go test -i ./mod/lock/v2/tests
go run third_party.go test -v ./mod/lock/v2/tests

go run third_party.go test -i ./tests/functional
ETCD_BIN_PATH=$(pwd)/bin/etcd go run third_party.go test -v ./tests/functional

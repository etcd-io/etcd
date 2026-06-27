#!/usr/bin/env bash

TEST=./...;
FMT="*.go"

echo "Running tests...";
go test -v -cover -cpu 1,2,4 $TEST;
go test -v -cover -cpu 1,2,4 -race $TEST;

echo "Checking gofmt..."
fmtRes=$(gofmt -l -s $FMT)
if [ -n "${fmtRes}" ]; then
	echo -e "gofmt checking failed:\n${fmtRes}"
	exit 255
fi

echo "Checking govet..."
vetRes=$(go vet $TEST)
if [ -n "${vetRes}" ]; then
	echo -e "govet checking failed:\n${vetRes}"
	exit 255
fi

echo "Success";


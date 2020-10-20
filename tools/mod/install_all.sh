#!/usr/bin/env bash

cd ./tools/mod || exit 2
go list --tags tools -f '{{ join .Imports "\n" }}' | xargs gobin -p

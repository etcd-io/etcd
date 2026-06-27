#!/usr/bin/env bash

GIT_SHA=$(git rev-parse --short HEAD || echo "GitNotFound")

# Set GO_LDFLAGS="-s" for building without symbols for debugging.
GO_LDFLAGS=("-X 'main.GitSHA=${GIT_SHA}'")
go build $GO_BUILD_FLAGS -ldflags="$GO_LDFLAGS" -o gofail

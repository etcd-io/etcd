VERSION ?= "unknown"
VERSION_FLAG := -X github.com/nunnatsa/ginkgolinter/version.version=$(VERSION)
COMMIT_HASH := $(shell git rev-parse HEAD)
HASH_FLAG := -X github.com/nunnatsa/ginkgolinter/version.gitHash=$(COMMIT_HASH)

BUILD_ARGS := -ldflags "$(VERSION_FLAG) $(HASH_FLAG)"

build: goimports
	go build $(BUILD_ARGS) -o ginkgolinter ./cmd/ginkgolinter

unit-test:
	go test ./...

build-for-windows:
	GOOS=windows GOARCH=amd64 go build $(BUILD_ARGS) -o bin/ginkgolinter-amd64.exe ./cmd/ginkgolinter
	GOOS=windows GOARCH=arm64 go build $(BUILD_ARGS) -o bin/ginkgolinter-arm64.exe ./cmd/ginkgolinter

build-for-mac:
	GOOS=darwin GOARCH=amd64 go build $(BUILD_ARGS) -o bin/ginkgolinter-amd64-darwin ./cmd/ginkgolinter
	GOOS=darwin GOARCH=arm64 go build $(BUILD_ARGS) -o bin/ginkgolinter-arm64-darwin ./cmd/ginkgolinter

build-for-linux:
	GOOS=linux GOARCH=amd64 go build $(BUILD_ARGS) -o bin/ginkgolinter-amd64-linux ./cmd/ginkgolinter
	GOOS=linux GOARCH=arm64 go build $(BUILD_ARGS) -o bin/ginkgolinter-arm64-linux ./cmd/ginkgolinter
	GOOS=linux GOARCH=386 go build $(BUILD_ARGS) -o bin/ginkgolinter-386-linux ./cmd/ginkgolinter

build-all: build build-for-linux build-for-mac build-for-windows

test-cli:
	RUN_CLI_TESTS=true go test -v ./tests/...

test: unit-test test-cli

goimports:
	go install golang.org/x/tools/cmd/goimports@latest
	goimports -w -local="github.com/nunnatsa/ginkgolinter" $(shell find . -type f -name '*.go' ! -path "*/vendor/*")

build-txtar-updater:
	go build -o bin/ ./tools/txtar_updater

update-txtar: build-txtar-updater
	bin/txtar_updater --target-dir="./tests/testdata" --source-dir="./testdata/src/a"

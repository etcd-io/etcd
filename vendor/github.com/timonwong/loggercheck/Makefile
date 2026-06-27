.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: test-deps
test-deps:
	cd testdata/src/a && go mod vendor

.PHONY: test
test: test-deps
	go test -v -covermode=atomic -coverprofile=cover.out -coverpkg ./... ./...

.PHONY: build
build:
	go build -o bin/loggercheck ./cmd/loggercheck

.PHONY: build-plugin
build-plugin:
	CGO_ENABLED=1 go build -o bin/loggercheck.so -buildmode=plugin ./plugin

.PHONY: build-all
build-all: build build-plugin

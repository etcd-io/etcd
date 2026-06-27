.PHONY: clean test build

default: test build

clean:
	rm -rf dist/ cover.out

test: clean
	go test -v -cover ./...

build:
	go  build -ldflags '-s -w' -o contextcheck ./cmd/contextcheck/main.go

install:
	go install -ldflags '-s -w' ./cmd/contextcheck

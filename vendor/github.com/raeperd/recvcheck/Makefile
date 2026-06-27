.PHONY: clean lint test build

default: clean lint test build

clean:
	rm -rf coverage.txt

build:
	go build -ldflags "-s -w" -trimpath ./cmd/recvcheck/

test: clean
	go test -race -coverprofile=coverage.txt .

lint:
	golangci-lint run


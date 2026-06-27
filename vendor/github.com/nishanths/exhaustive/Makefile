.PHONY: default
default: build

.PHONY: all
all: build vet test

.PHONY: build
build:
	go build ./...
	go build ./cmd/exhaustive

.PHONY: test
test:
	go test -count=1 ./...

.PHONY: testshort
testshort:
	go test -short -count=1 ./...

.PHONY: cover
cover:
	go test -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

.PHONY: install-vet
install-vet:
	go install github.com/nishanths/exhaustive/cmd/exhaustive@latest
	go install github.com/gordonklaus/ineffassign@latest
	go install github.com/kisielk/errcheck@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest

.PHONY: vet
vet:
	go vet ./...
	exhaustive ./...
	ineffassign ./...
	errcheck ./...
	staticcheck -checks="inherit,-S1034" ./...

.PHONY: upgrade-deps
upgrade-deps:
	go get golang.org/x/tools
	go mod tidy

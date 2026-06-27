current_dir = $(shell pwd)

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: build
build:
	go build -o "$$(go env GOPATH)/bin/gomodguard" cmd/gomodguard/main.go

.PHONY: run
run: build
	./gomodguard

.PHONY: test
test:
	go test -v -coverprofile coverage.out 

.PHONY: cover
cover:
	gocover-cobertura < coverage.out > coverage.xml

.PHONY: dockerrun
dockerrun: dockerbuild
	docker run -v "${current_dir}/.gomodguard.yaml:/.gomodguard.yaml" ryancurrah/gomodguard:latest

.PHONY: snapshot
snapshot:
	goreleaser --clean --snapshot

.PHONY: release
release:
	goreleaser --clean

.PHONY: clean
clean:
	rm -rf dist/
	rm -f gomodguard coverage.xml coverage.out

.PHONY: install-mac-tools
install-tools-mac:
	brew install goreleaser/tap/goreleaser

.PHONY: install-go-tools
install-go-tools:
	go install -v github.com/t-yuki/gocover-cobertura@latest

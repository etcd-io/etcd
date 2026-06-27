.EXPORT_ALL_VARIABLES:

VERSION  := $(shell git describe --abbrev=0 --tags | tr -d v)
COMMIT := $(shell git rev-parse --short HEAD)
LDFLAGS := -X 'main.version=$(VERSION)' \
           -X 'main.commit=$(COMMIT)'

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o dist/yamlfmt ./cmd/yamlfmt

.PHONY: test
test:
	go test ./...

.PHONY: test_v
test_v:
	@go test -v $$(go list ./... | grep -v "pkg/yaml")
	@go test ./pkg/yaml/formattest

.PHONY: vet
vet:
	go vet $$(go list ./... | grep -v "pkg/yaml")

YAMLFMT_BIN ?= $(shell pwd)/dist/yamlfmt
.PHONY: integrationtest
integrationtest:
	$(MAKE) build
	go test -tags=integration_test -run=$(TESTNAME) ./integrationtest/command

.PHONY: integrationtest_v
integrationtest_v:
	$(MAKE) build
	go test -v -tags=integration_test -run=$(TESTNAME) ./integrationtest/command

.PHONY: integrationtest_stdout
integrationtest_stdout:
	$(MAKE) build
	go test -v -tags=integration_test -run=$(TESTNAME) ./integrationtest/command -stdout

.PHONY: integrationtest_update
integrationtest_update:
	$(MAKE) build
	go test -tags=integration_test -v ./integrationtest/command -update

.PHONY: command_test_case
command_test_case:
ifndef TESTNAME
	$(error "TESTNAME undefined")
endif
	./integrationtest/command/new_test_case.sh "$(TESTNAME)"

.PHONY: install
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/yamlfmt

.PHONY: install_tools
install_tools:
	go install github.com/google/addlicense@latest

ADDLICENSE = addlicense -ignore "**/testdata/**" -ignore "**/pkg/yaml/**" -c "Google LLC" -l apache

.PHONY: addlicense
addlicense:
	$(ADDLICENSE) .

.PHONY: addlicense_check
addlicense_check:
	$(ADDLICENSE) -check .

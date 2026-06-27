GIT_TAG?= $(shell git describe --always --tags)
BIN = zxcvbn-go
FMT_CMD = $(gofmt -s -l -w $(find . -type f -name '*.go' -not -path './vendor/*') | tee /dev/stderr)
IMAGE_REPO = ccojocar
DATE_FMT=+%Y-%m-%d
ifdef SOURCE_DATE_EPOCH
    BUILD_DATE ?= $(shell date -u -d "@$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u "$(DATE_FMT)")
else
    BUILD_DATE ?= $(shell date "$(DATE_FMT)")
endif
BUILDFLAGS := "-w -s -X 'main.Version=$(GIT_TAG)' -X 'main.GitTag=$(GIT_TAG)' -X 'main.BuildDate=$(BUILD_DATE)'"
CGO_ENABLED = 0
GO := GO111MODULE=on go
GO_NOMOD :=GO111MODULE=off go
GOPATH ?= $(shell $(GO) env GOPATH)
GOBIN ?= $(GOPATH)/bin
GO_MINOR_VERSION = $(shell $(GO) version | cut -c 14- | cut -d' ' -f1 | cut -d'.' -f2)
GOVULN_MIN_VERSION = 17
GO_VERSION = 1.20

default: 
	$(MAKE) test

install-govulncheck:
	@if [ $(GO_MINOR_VERSION) -gt $(GOVULN_MIN_VERSION) ]; then \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi

test-all: fmt vet lint sec govulncheck test

test:
	go test -v ./...

fmt:
	@echo "FORMATTING"
	@FORMATTED=`$(GO) fmt ./...`
	@([ ! -z "$(FORMATTED)" ] && printf "Fixed unformatted files:\n$(FORMATTED)") || true

vet:
	@echo "VETTING"
	$(GO) vet ./...

lint:
	@echo "LINTING: golangci-lint"
	golangci-lint run

sec:
	@echo "SECURITY SCANNING"
	gosec ./...

govulncheck: install-govulncheck
	@echo "CHECKING VULNERABILITIES"
	@if [ $(GO_MINOR_VERSION) -gt $(GOVULN_MIN_VERSION) ]; then \
		govulncheck ./...; \
	fi

clean:
	rm -rf build vendor dist coverage.txt
	rm -f release image $(BIN)

.PHONY: test test-all fmt vet govulncheck clean

GIT_TAG?= $(shell git describe --always --tags)
BIN = gosec
FMT_CMD = $(gofmt -s -l -w $(find . -type f -name '*.go' -not -path './vendor/*') | tee /dev/stderr)
IMAGE_REPO = securego
DATE_FMT=+%Y-%m-%d
ifdef SOURCE_DATE_EPOCH
    BUILD_DATE ?= $(shell date -u -d "@$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u "$(DATE_FMT)")
else
    BUILD_DATE ?= $(shell date "$(DATE_FMT)")
endif
BUILDFLAGS := "-w -s -X 'main.Version=$(GIT_TAG)' -X 'main.GitTag=$(GIT_TAG)' -X 'main.BuildDate=$(BUILD_DATE)'"
CGO_ENABLED = 0
GO := GO111MODULE=on go
GOPATH ?= $(shell $(GO) env GOPATH)
GOBIN ?= $(GOPATH)/bin
GOSEC ?= $(GOBIN)/gosec
GO_MINOR_VERSION = $(shell $(GO) version | cut -c 14- | cut -d' ' -f1 | cut -d'.' -f2)
GOVULN_MIN_VERSION = 17
GO_VERSION = 1.26
LDFLAGS = -ldflags "\
	-X 'main.Version=$(shell git describe --tags --always)' \
	-X 'main.GitTag=$(shell git describe --tags --abbrev=0)' \
	-X 'main.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)'"

default:
	$(MAKE) build

install-govulncheck:
	@if [ $(GO_MINOR_VERSION) -gt $(GOVULN_MIN_VERSION) ]; then \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi

test: build-race fmt vet sec govulncheck
	go run github.com/onsi/ginkgo/v2/ginkgo -- --ginkgo.v --ginkgo.fail-fast

test-nocache: build-race fmt vet sec govulncheck
	go test -count=1 -v ./...

fmt:
	@echo "FORMATTING"
	@FORMATTED=`$(GO) fmt ./...`
	@([ ! -z "$(FORMATTED)" ] && printf "Fixed unformatted files:\n$(FORMATTED)") || true

vet:
	@echo "VETTING"
	$(GO) vet ./...

golangci:
	@echo "LINTING: golangci-lint"
	golangci-lint run

sec:
	@echo "SECURITY SCANNING"
	./$(BIN) -exclude-dir=testdata ./...

govulncheck: install-govulncheck
	@echo "CHECKING VULNERABILITIES"
	@if [ $(GO_MINOR_VERSION) -gt $(GOVULN_MIN_VERSION) ]; then \
		govulncheck ./...; \
	fi

test-coverage:
	go test -race -v -count=1 -coverpkg=./... -coverprofile=coverage.out ./...

build:
	go build $(LDFLAGS) -o $(BIN) ./cmd/gosec/

build-race:
	go build -race $(LDFLAGS) -o $(BIN) ./cmd/gosec/

build-debug:
	go build -tags debug $(LDFLAGS) -o $(BIN)-debug ./cmd/gosec/

build-debug-race:
	go build -race -tags debug $(LDFLAGS) -o $(BIN)-debug ./cmd/gosec/

clean:
	rm -rf build vendor dist coverage.out
	rm -f release image $(BIN) $(BIN)-debug

release:
	@echo "Releasing the gosec binary..."
	goreleaser release

build-linux:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux go build -ldflags=$(BUILDFLAGS) -o $(BIN) ./cmd/gosec/

image:
	@echo "Building the Docker image..."
	docker build -t $(IMAGE_REPO)/$(BIN):$(GIT_TAG) --build-arg GO_VERSION=$(GO_VERSION) .
	docker tag $(IMAGE_REPO)/$(BIN):$(GIT_TAG) $(IMAGE_REPO)/$(BIN):latest
	touch image

image-push: image
	@echo "Pushing the Docker image..."
	docker push $(IMAGE_REPO)/$(BIN):$(GIT_TAG)
	docker push $(IMAGE_REPO)/$(BIN):latest

tlsconfig:
	go generate ./...

perf-diff:
	./perf-diff.sh

.PHONY: test test-nocache build clean release image image-push tlsconfig perf-diff

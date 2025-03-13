REPOSITORY_ROOT := $(shell git rev-parse --show-toplevel)

.PHONY: all
all: build
include $(REPOSITORY_ROOT)/tests/robustness/Makefile

.PHONY: build
build:
	GO_BUILD_FLAGS="${GO_BUILD_FLAGS} -v -mod=readonly" ./scripts/build.sh

PLATFORMS=linux-amd64 linux-386 linux-arm linux-arm64 linux-ppc64le linux-s390x darwin-amd64 darwin-arm64 windows-amd64 windows-arm64

.PHONY: build-all
build-all:
	@for platform in $(PLATFORMS); do \
		$(MAKE) build-$${platform}; \
	done

.PHONY: build-%
build-%:
	GOOS=$$(echo $* | cut -d- -f 1) GOARCH=$$(echo $* | cut -d- -f 2) GO_BUILD_FLAGS="${GO_BUILD_FLAGS} -v -mod=readonly" ./scripts/build.sh

.PHONY: tools
tools:
	GO_BUILD_FLAGS="${GO_BUILD_FLAGS} -v -mod=readonly" ./scripts/build_tools.sh

# Tests

GO_TEST_FLAGS?=

.PHONY: test
test:
	PASSES="unit integration release e2e" ./scripts/test.sh $(GO_TEST_FLAGS)

.PHONY: test-unit
test-unit:
	PASSES="unit" ./scripts/test.sh $(GO_TEST_FLAGS)

.PHONY: test-integration
test-integration:
	PASSES="integration" ./scripts/test.sh $(GO_TEST_FLAGS)

.PHONY: test-e2e
test-e2e: build
	PASSES="e2e" ./scripts/test.sh $(GO_TEST_FLAGS)

.PHONY: test-grpcproxy-integration
test-grpcproxy-integration:
	PASSES="grpcproxy_integration" ./scripts/test.sh $(GO_TEST_FLAGS)

.PHONY: test-grpcproxy-e2e
test-grpcproxy-e2e: build
	PASSES="grpcproxy_e2e" ./scripts/test.sh $(GO_TEST_FLAGS)

.PHONY: test-e2e-release
test-e2e-release: build
	PASSES="release e2e" ./scripts/test.sh $(GO_TEST_FLAGS)

.PHONY: test-robustness
test-robustness:
	PASSES="robustness" ./scripts/test.sh $(GO_TEST_FLAGS)

.PHONY: test-coverage
test-coverage:
	COVERDIR=covdir PASSES="build cov" ./scripts/test.sh $(GO_TEST_FLAGS)

.PHONY: upload-coverage-report
upload-coverage-report: test-coverage
	./scripts/codecov_upload.sh

.PHONY: fuzz
fuzz: 
	./scripts/fuzzing.sh

# Static analysis
.PHONY: verify
verify: verify-gofmt verify-bom verify-lint verify-dep verify-shellcheck verify-goword \
	verify-govet verify-license-header verify-mod-tidy \
	verify-shellws verify-proto-annotations verify-genproto verify-yamllint \
	verify-govet-shadow verify-markdown-marker verify-go-versions

.PHONY: fix
fix: fix-bom fix-lint fix-yamllint sync-toolchain-directive
	./scripts/fix.sh

.PHONY: verify-gofmt
verify-gofmt:
	PASSES="gofmt" ./scripts/test.sh

.PHONY: verify-bom
verify-bom:
	PASSES="bom" ./scripts/test.sh

.PHONY: fix-bom
fix-bom:
	./scripts/updatebom.sh

.PHONY: verify-dep
verify-dep:
	PASSES="dep" ./scripts/test.sh

.PHONY: verify-lint
verify-lint: install-golangci-lint
	PASSES="lint" ./scripts/test.sh

.PHONY: fix-lint
fix-lint:
	PASSES="lint_fix" ./scripts/test.sh

.PHONY: verify-shellcheck
verify-shellcheck:
	PASSES="shellcheck" ./scripts/test.sh

.PHONY: verify-goword
verify-goword:
	PASSES="goword" ./scripts/test.sh

.PHONY: verify-govet
verify-govet:
	PASSES="govet" ./scripts/test.sh

.PHONY: verify-license-header
verify-license-header:
	PASSES="license_header" ./scripts/test.sh

.PHONY: verify-mod-tidy
verify-mod-tidy:
	PASSES="mod_tidy" ./scripts/test.sh

.PHONY: verify-shellws
verify-shellws:
	PASSES="shellws" ./scripts/test.sh

.PHONY: verify-proto-annotations
verify-proto-annotations:
	PASSES="proto_annotations" ./scripts/test.sh

.PHONY: verify-genproto
verify-genproto:
	PASSES="genproto" ./scripts/test.sh

.PHONY: verify-yamllint
verify-yamllint:
ifeq (, $(shell which yamllint))
	@echo "Installing yamllint..."
	tmpdir=$$(mktemp -d); \
	trap "rm -rf $$tmpdir" EXIT; \
	python3 -m venv $$tmpdir; \
	$$tmpdir/bin/python3 -m pip install yamllint; \
	$$tmpdir/bin/yamllint --config-file tools/.yamllint .
else
	@echo "yamllint already installed..."
	yamllint --config-file tools/.yamllint .
endif

.PHONY: verify-govet-shadow
verify-govet-shadow:
	PASSES="govet_shadow" ./scripts/test.sh

.PHONY: verify-markdown-marker
verify-markdown-marker:
	PASSES="markdown_marker" ./scripts/test.sh

YAMLFMT_VERSION = $(shell cd tools/mod && go list -m -f '{{.Version}}' github.com/google/yamlfmt)

.PHONY: fix-yamllint
fix-yamllint:
ifeq (, $(shell which yamlfmt))
	$(shell go install github.com/google/yamlfmt/cmd/yamlfmt@$(YAMLFMT_VERSION))
endif
	yamlfmt -conf tools/.yamlfmt .

.PHONY: run-govulncheck
run-govulncheck:
ifeq (, $(shell which govulncheck))
	$(shell go install golang.org/x/vuln/cmd/govulncheck@latest)
endif
	PASSES="govuln" ./scripts/test.sh

# Tools

GOLANGCI_LINT_VERSION = $(shell cd tools/mod && go list -m -f {{.Version}} github.com/golangci/golangci-lint)
.PHONY: install-golangci-lint
install-golangci-lint:
ifeq (, $(shell which golangci-lint))
	$(shell curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin $(GOLANGCI_LINT_VERSION))
endif

.PHONY: install-lazyfs
install-lazyfs: bin/lazyfs
bin/lazyfs:
	rm /tmp/lazyfs -rf
	git clone --depth 1 --branch 0.2.0 https://github.com/dsrhaslab/lazyfs /tmp/lazyfs
	cd /tmp/lazyfs/libs/libpcache; ./build.sh
	cd /tmp/lazyfs/lazyfs; ./build.sh
	mkdir -p ./bin
	cp /tmp/lazyfs/lazyfs/build/lazyfs ./bin/lazyfs

# Cleanup
.PHONY: clean
clean:
	rm -f ./codecov
	rm -rf ./covdir
	rm -f ./bin/Dockerfile-release
	rm -rf ./bin/etcd*
	rm -rf ./bin/lazyfs
	rm -rf ./bin/python
	rm -rf ./default.etcd
	rm -rf ./tests/e2e/default.etcd
	rm -rf ./release
	rm -rf ./coverage/*.err ./coverage/*.out
	rm -rf ./tests/e2e/default.proxy
	rm -rf ./bin/shellcheck*
	find ./ -name "127.0.0.1:*" -o -name "localhost:*" -o -name "*.log" -o -name "agent-*" -o -name "*.coverprofile" -o -name "testname-proxy-*" -delete

.PHONY: verify-go-versions
verify-go-versions:
	./scripts/verify_go_versions.sh

.PHONY: sync-toolchain-directive
sync-toolchain-directive:
	./scripts/sync_go_toolchain_directive.sh

.PHONY: markdown-diff-lint
markdown-diff-lint:
	./scripts/markdown_diff_lint.sh

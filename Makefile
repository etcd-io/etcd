all: build
include tests/robustness/makefile.mk

.PHONY: build
build:
	GO_BUILD_FLAGS="${GO_BUILD_FLAGS} -v -mod=readonly" ./scripts/build.sh

.PHONY: tools
tools:
	GO_BUILD_FLAGS="${GO_BUILD_FLAGS} -v -mod=readonly" ./scripts/build_tools.sh

TEMP_TEST_ANALYZER_DIR=/tmp/etcd-test-analyzer
TEST_ANALYZER_BIN=${PWD}/bin
bin/etcd-test-analyzer: $(TEMP_TEST_ANALYZER_DIR)/*
	make -C ${TEMP_TEST_ANALYZER_DIR} build
	mkdir -p ${TEST_ANALYZER_BIN}
	install ${TEMP_TEST_ANALYZER_DIR}/bin/etcd-test-analyzer ${TEST_ANALYZER_BIN}
	${TEST_ANALYZER_BIN}/etcd-test-analyzer -h

$(TEMP_TEST_ANALYZER_DIR)/*:
	git clone "https://github.com/endocrimes/etcd-test-analyzer.git" ${TEMP_TEST_ANALYZER_DIR}

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

.PHONY: fuzz
fuzz: 
	./scripts/fuzzing.sh

# Static analysis

verify: verify-gofmt verify-bom verify-lint verify-dep verify-shellcheck verify-goword \
	verify-govet verify-license-header verify-receiver-name verify-mod-tidy verify-shellcheck \
	verify-shellws verify-proto-annotations verify-genproto verify-goimport verify-yamllint
fix: fix-goimports fix-bom fix-lint fix-yamllint
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
verify-lint:
	golangci-lint run --config tools/.golangci.yaml

.PHONY: fix-lint
fix-lint:
	golangci-lint run --config tools/.golangci.yaml --fix

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

.PHONY: verify-receiver-name
verify-receiver-name:
	PASSES="receiver_name" ./scripts/test.sh

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

.PHONY: verify-goimport
verify-goimport:
	PASSES="goimport" ./scripts/test.sh

.PHONY: fix-goimports
fix-goimports:
	./scripts/fix-goimports.sh

.PHONY: verify-yamllint
verify-yamllint:
	yamllint --config-file tools/.yamllint .

YAMLFMT_VERSION = $(shell cd tools/mod && go list -m -f '{{.Version}}' github.com/google/yamlfmt)

.PHONY: fix-yamllint
fix-yamllint:
ifeq (, $(shell which yamlfmt))
	$(shell go install github.com/google/yamlfmt/cmd/yamlfmt@$(YAMLFMT_VERSION))
endif
	yamlfmt -conf tools/.yamlfmt .

# Tools

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

clean:
	rm -f ./codecov
	rm -rf ./covdir
	rm -f ./bin/Dockerfile-release
	rm -rf ./bin/etcd*
	rm -rf ./bin/lazyfs
	rm -rf ./default.etcd
	rm -rf ./tests/e2e/default.etcd
	rm -rf ./release
	rm -rf ./coverage/*.err ./coverage/*.out
	rm -rf ./tests/e2e/default.proxy
	rm -rf ./bin/shellcheck*
	find ./ -name "127.0.0.1:*" -o -name "localhost:*" -o -name "*.log" -o -name "agent-*" -o -name "*.coverprofile" -o -name "testname-proxy-*" -delete

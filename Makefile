.PHONY: build
build:
	GO_BUILD_FLAGS="-v" ./scripts/build.sh
	./bin/etcd --version
	./bin/etcdctl version
	./bin/etcdutl version

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

.PHONY: test-e2e-release
test-e2e-release: build
	PASSES="release e2e" ./scripts/test.sh $(GO_TEST_FLAGS)

.PHONY: test-linearizability
test-linearizability:
	FAILPOINTS=true PASSES="linearizability" ./scripts/test.sh $(GO_TEST_FLAGS)

.PHONY: fuzz
fuzz: 
	./scripts/fuzzing.sh

# Static analysis

verify: verify-gofmt verify-bom verify-lint verify-dep verify-shellcheck verify-goword \
	verify-govet verify-license-header verify-receiver-name verify-mod-tidy verify-shellcheck \
	verify-shellws verify-proto-annotations verify-genproto
fix: fix-bom fix-lint
	./scripts/fix.sh

.PHONY: verify-gofmt
verify-gofmt:
	PASSES="gofmt" ./scripts/test.sh

.PHONY: verify-bom
verify-bom:
	PASSES="bom" ./scripts/test.sh

.PHONY: update-bom
fix-bom:
	./scripts/updatebom.sh

.PHONY: verify-dep
verify-dep:
	PASSES="dep" ./scripts/test.sh

.PHONY: verify-lint
verify-lint:
	golangci-lint run

.PHONY: update-lint
fix-lint:
	golangci-lint run --fix

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

# Old release builds for reproducing issues

.PHONY: release-issue14370
release-issue14370: ./bin/etcd-v3.5.4-failpoints
	cp ./bin/etcd-v3.5.4-failpoints ./bin/etcd

./bin/etcd-v3.5.4-failpoints:
	rm -rf /tmp/etcd-release-issue14370/
	mkdir -p /tmp/etcd-release-issue14370/
	cd /tmp/etcd-release-issue14370/; \
	  git clone https://github.com/etcd-io/etcd.git .; \
	  git checkout v3.5.4; \
	  go get go.etcd.io/gofail/runtime; \
	  (cd server; go get go.etcd.io/gofail/runtime); \
	  (cd etcdctl; go get go.etcd.io/gofail/runtime); \
	  (cd etcdutl; go get go.etcd.io/gofail/runtime); \
	  FAILPOINTS=true make build;
	mkdir -p ./bin
	cp /tmp/etcd-release-issue14370/bin/etcd ./bin/etcd-v3.5.4-failpoints

# Cleanup

clean:
	rm -f ./codecov
	rm -rf ./covdir
	rm -f ./bin/Dockerfile-release*
	rm -rf ./bin/etcd*
	rm -rf ./default.etcd
	rm -rf ./tests/e2e/default.etcd
	rm -rf ./release
	rm -rf ./coverage/*.err ./coverage/*.out
	rm -rf ./tests/e2e/default.proxy
	find ./ -name "127.0.0.1:*" -o -name "localhost:*" -o -name "*.log" -o -name "agent-*" -o -name "*.coverprofile" -o -name "testname-proxy-*" -delete

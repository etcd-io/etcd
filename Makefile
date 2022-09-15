.PHONY: build
build:
	GO_BUILD_FLAGS="-v" ./scripts/build.sh
	./bin/etcd --version
	./bin/etcdctl version
	./bin/etcdutl version

# Tests

.PHONY: test
test:
	PASSES="unit integration release e2e" ./scripts/test.sh

.PHONY: test-unit
test-unit:
	PASSES="unit" ./scripts/test.sh

.PHONY: test-integration
test-integration:
	PASSES="integration" ./scripts/test.sh

.PHONY: test-e2e
test-e2e: build
	PASSES="e2e" ./scripts/test.sh

.PHONY: test-e2e-release
test-e2e-release: build
	PASSES="release e2e" ./scripts/test.sh

# Static analysis

verify: verify-fmt verify-bom verify-lint verify-dep
update: update-bom update-lint update-dep update-fix

.PHONY: verify-fmt
verify-fmt:
	PASSES="fmt" ./scripts/test.sh

.PHONY: verify-bom
verify-bom:
	PASSES="bom" ./scripts/test.sh

.PHONY: update-bom
update-bom:
	./scripts/updatebom.sh

.PHONY: verify-dep
verify-dep:
	PASSES="dep" ./scripts/test.sh

.PHONY: update-dep
update-dep:
	./scripts/update_dep.sh

.PHONY: verify-lint
verify-lint:
	golangci-lint run

.PHONY: update-lint
update-lint:
	golangci-lint run --fix

.PHONY: update-fix
update-fix:
	./scripts/fix.sh

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

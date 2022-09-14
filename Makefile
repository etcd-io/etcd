.PHONY: build
build:
	GO_BUILD_FLAGS="-v" ./scripts/build.sh
	./bin/etcd --version
	./bin/etcdctl version
	./bin/etcdutl version

.PHONY: test-fmt
test-fmt:
	PASSES="fmt" ./scripts/test.sh

.PHONY: test-bom
test-bom:
	PASSES="bom" ./scripts/test.sh

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

.PHONY: test-all
test-all:
	PASSES="fmt bom dep unit integration release e2e" ./scripts/test.sh

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

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

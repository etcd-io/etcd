# Test previous release branches

.PHONY: test-robustness-release-3.5
test-robustness-release-3.5: /tmp/etcd-release-3.5-failpoints/bin
	GO_TEST_FLAGS="$${GO_TEST_FLAGS} --bin-dir=/tmp/etcd-release-3.5-failpoints/bin" make test-robustness

.PHONY: test-robustness-release-3.4
test-robustness-release-3.4: /tmp/etcd-release-3.4-failpoints/bin
	GO_TEST_FLAGS="$${GO_TEST_FLAGS} --bin-dir=/tmp/etcd-release-3.4-failpoints/bin" make test-robustness

# Reproduce historical issues

.PHONY: test-robustness-issue14370
test-robustness-issue14370: /tmp/etcd-v3.5.4-failpoints/bin
	GO_TEST_FLAGS='-v --run=TestRobustnessRegression/Issue14370 --count 100 --failfast --bin-dir=/tmp/etcd-v3.5.4-failpoints/bin' make test-robustness && \
	 echo "Failed to reproduce" || echo "Successful reproduction"

.PHONY: test-robustness-issue13766
test-robustness-issue13766: /tmp/etcd-v3.5.2-failpoints/bin
	GO_TEST_FLAGS='-v --run=TestRobustnessRegression/Issue13766 --count 100 --failfast --bin-dir=/tmp/etcd-v3.5.2-failpoints/bin' make test-robustness && \
	 echo "Failed to reproduce" || echo "Successful reproduction"

.PHONY: test-robustness-issue14685
test-robustness-issue14685: /tmp/etcd-v3.5.5-failpoints/bin
	GO_TEST_FLAGS='-v --run=TestRobustnessRegression/Issue14685 --count 100 --failfast --bin-dir=/tmp/etcd-v3.5.5-failpoints/bin' make test-robustness && \
	 echo "Failed to reproduce" || echo "Successful reproduction"

.PHONY: test-robustness-issue15271
test-robustness-issue15271: /tmp/etcd-v3.5.7-failpoints/bin
	GO_TEST_FLAGS='-v --run=TestRobustnessRegression/Issue15271 --count 100 --failfast --bin-dir=/tmp/etcd-v3.5.7-failpoints/bin' make test-robustness && \
	 echo "Failed to reproduce" || echo "Successful reproduction"

# Failpoints

GOFAIL_VERSION = $(shell cd tools/mod && go list -m -f {{.Version}} go.etcd.io/gofail)

.PHONY: gofail-enable
gofail-enable: install-gofail
	gofail enable server/etcdserver/ server/storage/backend/ server/storage/mvcc/ server/storage/wal/ server/etcdserver/api/v3rpc/
	cd ./server && go get go.etcd.io/gofail@${GOFAIL_VERSION}
	cd ./etcdutl && go get go.etcd.io/gofail@${GOFAIL_VERSION}
	cd ./etcdctl && go get go.etcd.io/gofail@${GOFAIL_VERSION}
	cd ./tests && go get go.etcd.io/gofail@${GOFAIL_VERSION}

.PHONY: gofail-disable
gofail-disable: install-gofail
	gofail disable server/etcdserver/ server/storage/backend/ server/storage/mvcc/ server/storage/wal/ server/etcdserver/api/v3rpc/
	cd ./server && go mod tidy
	cd ./etcdutl && go mod tidy
	cd ./etcdctl && go mod tidy
	cd ./tests && go mod tidy

.PHONY: install-gofail
install-gofail:
	go install go.etcd.io/gofail@${GOFAIL_VERSION}

# Build previous releases for robustness tests

/tmp/etcd-v3.6.0-failpoints/bin: install-gofail
	rm -rf /tmp/etcd-v3.6.0-failpoints/
	mkdir -p /tmp/etcd-v3.6.0-failpoints/
	cd /tmp/etcd-v3.6.0-failpoints/; \
	  git clone --depth 1 --branch main https://github.com/etcd-io/etcd.git .; \
	  make gofail-enable; \
	  make build;

/tmp/etcd-v3.5.2-failpoints/bin:
/tmp/etcd-v3.5.4-failpoints/bin:
/tmp/etcd-v3.5.5-failpoints/bin:
/tmp/etcd-v3.5.%-failpoints/bin: install-gofail
	rm -rf /tmp/etcd-v3.5.$*-failpoints/
	mkdir -p /tmp/etcd-v3.5.$*-failpoints/
	cd /tmp/etcd-v3.5.$*-failpoints/; \
	  git clone --depth 1 --branch v3.5.$* https://github.com/etcd-io/etcd.git .; \
	  go get go.etcd.io/gofail@${GOFAIL_VERSION}; \
	  (cd server; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  (cd etcdctl; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  (cd etcdutl; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  FAILPOINTS=true ./build;

/tmp/etcd-release-3.5-failpoints/bin: install-gofail
	rm -rf /tmp/etcd-release-3.5-failpoints/
	mkdir -p /tmp/etcd-release-3.5-failpoints/
	cd /tmp/etcd-release-3.5-failpoints/; \
	  git clone --depth 1 --branch release-3.5 https://github.com/etcd-io/etcd.git .; \
	  go get go.etcd.io/gofail@${GOFAIL_VERSION}; \
	  (cd server; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  (cd etcdctl; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  (cd etcdutl; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  FAILPOINTS=true ./build;

/tmp/etcd-v3.4.23-failpoints/bin:
/tmp/etcd-v3.4.%-failpoints/bin: install-gofail
	rm -rf /tmp/etcd-v3.4.$*-failpoints/
	mkdir -p /tmp/etcd-v3.4.$*-failpoints/
	cd /tmp/etcd-v3.4.$*-failpoints/; \
	  git clone --depth 1 --branch v3.4.$* https://github.com/etcd-io/etcd.git .; \
	  go get go.etcd.io/gofail@${GOFAIL_VERSION}; \
	  FAILPOINTS=true ./build;

/tmp/etcd-release-3.4-failpoints/bin: install-gofail
	rm -rf /tmp/etcd-release-3.4-failpoints/
	mkdir -p /tmp/etcd-release-3.4-failpoints/
	cd /tmp/etcd-release-3.4-failpoints/; \
	  git clone --depth 1 --branch release-3.4 https://github.com/etcd-io/etcd.git .; \
	  go get go.etcd.io/gofail@${GOFAIL_VERSION}; \
	  FAILPOINTS=true ./build;

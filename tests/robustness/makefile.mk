.PHONY: test-robustness-reports
test-robustness-reports: export GOTOOLCHAIN := go$(shell cat .go-version)
test-robustness-reports:
	cd ./tests && go test ./robustness/validate -v --count 1 --run TestDataReports

# Test main and previous release branches

.PHONY: test-robustness-main
test-robustness-main: /tmp/etcd-main-failpoints/bin /tmp/etcd-release-3.5-failpoints/bin
	GO_TEST_FLAGS="$${GO_TEST_FLAGS} --bin-dir=/tmp/etcd-main-failpoints/bin --bin-last-release=/tmp/etcd-release-3.5-failpoints/bin/etcd" $(MAKE) test-robustness

.PHONY: test-robustness-release-3.5
test-robustness-release-3.5: /tmp/etcd-release-3.5-failpoints/bin /tmp/etcd-release-3.4-failpoints/bin
	GO_TEST_FLAGS="$${GO_TEST_FLAGS} --bin-dir=/tmp/etcd-release-3.5-failpoints/bin --bin-last-release=/tmp/etcd-release-3.4-failpoints/bin/etcd" $(MAKE) test-robustness

.PHONY: test-robustness-release-3.4
test-robustness-release-3.4: /tmp/etcd-release-3.4-failpoints/bin
	GO_TEST_FLAGS="$${GO_TEST_FLAGS} --bin-dir=/tmp/etcd-release-3.4-failpoints/bin" $(MAKE) test-robustness

# Reproduce historical issues

.PHONY: test-robustness-issue14370
test-robustness-issue14370: /tmp/etcd-v3.5.4-failpoints/bin
	GO_TEST_FLAGS='-v --run=TestRobustnessRegression/Issue14370 --count 100 --failfast --bin-dir=/tmp/etcd-v3.5.4-failpoints/bin' $(MAKE) test-robustness && \
	 echo "Failed to reproduce" || echo "Successful reproduction"

.PHONY: test-robustness-issue13766
test-robustness-issue13766: /tmp/etcd-v3.5.2-failpoints/bin
	GO_TEST_FLAGS='-v --run=TestRobustnessRegression/Issue13766 --count 100 --failfast --bin-dir=/tmp/etcd-v3.5.2-failpoints/bin' $(MAKE) test-robustness && \
	 echo "Failed to reproduce" || echo "Successful reproduction"

.PHONY: test-robustness-issue14685
test-robustness-issue14685: /tmp/etcd-v3.5.5-failpoints/bin
	GO_TEST_FLAGS='-v --run=TestRobustnessRegression/Issue14685 --count 100 --failfast --bin-dir=/tmp/etcd-v3.5.5-failpoints/bin' $(MAKE) test-robustness && \
	 echo "Failed to reproduce" || echo "Successful reproduction"

.PHONY: test-robustness-issue15271
test-robustness-issue15271: /tmp/etcd-v3.5.7-failpoints/bin
	GO_TEST_FLAGS='-v --run=TestRobustnessRegression/Issue15271 --count 100 --failfast --bin-dir=/tmp/etcd-v3.5.7-failpoints/bin' $(MAKE) test-robustness && \
	 echo "Failed to reproduce" || echo "Successful reproduction"

.PHONY: test-robustness-issue17529
test-robustness-issue17529: /tmp/etcd-v3.5.12-beforeSendWatchResponse/bin
	GO_TEST_FLAGS='-v --run=TestRobustnessRegression/Issue17529 --count 100 --failfast --bin-dir=/tmp/etcd-v3.5.12-beforeSendWatchResponse/bin' $(MAKE) test-robustness && \
	 echo "Failed to reproduce" || echo "Successful reproduction"

.PHONY: test-robustness-issue17780
test-robustness-issue17780: /tmp/etcd-v3.5.13-compactBeforeSetFinishedCompact/bin
	GO_TEST_FLAGS='-v --run=TestRobustnessRegression/Issue17780 --count 200 --failfast --bin-dir=/tmp/etcd-v3.5.13-compactBeforeSetFinishedCompact/bin' make test-robustness && \
	  echo "Failed to reproduce" || echo "Successful reproduction"

.PHONY: test-robustness-issue18089
test-robustness-issue18089: /tmp/etcd-v3.5.12-beforeSendWatchResponse/bin
	GO_TEST_FLAGS='-v -run=TestRobustnessRegression/Issue18089 -count 100 -failfast --bin-dir=/tmp/etcd-v3.5.12-beforeSendWatchResponse/bin' make test-robustness && \
	  echo "Failed to reproduce" || echo "Successful reproduction"

.PHONY: test-robustness-issue19179
test-robustness-issue19179: /tmp/etcd-v3.5.17-failpoints/bin
	GO_TEST_FLAGS='-v -run=TestRobustnessRegression/Issue19179 -count 200 -failfast --bin-dir=/tmp/etcd-v3.5.17-failpoints/bin' make test-robustness && \
	  echo "Failed to reproduce" || echo "Successful reproduction"

# Failpoints

GOPATH = $(shell go env GOPATH)
GOFAIL_VERSION = $(shell cd tools/mod && go list -m -f {{.Version}} go.etcd.io/gofail)

.PHONY:install-gofail
install-gofail: $(GOPATH)/bin/gofail

.PHONY: gofail-enable
gofail-enable: $(GOPATH)/bin/gofail
	$(GOPATH)/bin/gofail enable server/etcdserver/ server/lease server/lease/leasehttp server/storage/backend/ server/storage/mvcc/ server/storage/wal/ server/etcdserver/api/v3rpc/ server/etcdserver/api/membership/
	cd ./server && go get go.etcd.io/gofail@${GOFAIL_VERSION}
	cd ./etcdutl && go get go.etcd.io/gofail@${GOFAIL_VERSION}
	cd ./etcdctl && go get go.etcd.io/gofail@${GOFAIL_VERSION}
	cd ./tests && go get go.etcd.io/gofail@${GOFAIL_VERSION}

.PHONY: gofail-disable
gofail-disable: $(GOPATH)/bin/gofail
	$(GOPATH)/bin/gofail disable server/etcdserver/ server/lease server/lease/leasehttp server/storage/backend/ server/storage/mvcc/ server/storage/wal/ server/etcdserver/api/v3rpc/ server/etcdserver/api/membership/
	cd ./server && go mod tidy
	cd ./etcdutl && go mod tidy
	cd ./etcdctl && go mod tidy
	cd ./tests && go mod tidy

$(GOPATH)/bin/gofail: tools/mod/go.mod tools/mod/go.sum
	go install go.etcd.io/gofail@${GOFAIL_VERSION}

# Build main and previous releases for robustness tests

/tmp/etcd-main-failpoints/bin: $(GOPATH)/bin/gofail
	rm -rf /tmp/etcd-main-failpoints/
	mkdir -p /tmp/etcd-main-failpoints/
	cd /tmp/etcd-main-failpoints/; \
	  git clone --depth 1 --branch main https://github.com/etcd-io/etcd.git .; \
	  $(MAKE) gofail-enable; \
	  $(MAKE) build;

/tmp/etcd-v3.6.0-failpoints/bin: $(GOPATH)/bin/gofail
	rm -rf /tmp/etcd-v3.6.0-failpoints/
	mkdir -p /tmp/etcd-v3.6.0-failpoints/
	cd /tmp/etcd-v3.6.0-failpoints/; \
	  git clone --depth 1 --branch main https://github.com/etcd-io/etcd.git .; \
	  $(MAKE) gofail-enable; \
	  $(MAKE) build;

/tmp/etcd-v3.5.2-failpoints/bin:
/tmp/etcd-v3.5.4-failpoints/bin:
/tmp/etcd-v3.5.5-failpoints/bin:
/tmp/etcd-v3.5.%-failpoints/bin: $(GOPATH)/bin/gofail
	rm -rf /tmp/etcd-v3.5.$*-failpoints/
	mkdir -p /tmp/etcd-v3.5.$*-failpoints/
	cd /tmp/etcd-v3.5.$*-failpoints/; \
	  git clone --depth 1 --branch v3.5.$* https://github.com/etcd-io/etcd.git .; \
	  go get go.etcd.io/gofail@${GOFAIL_VERSION}; \
	  (cd server; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  (cd etcdctl; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  (cd etcdutl; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  (cd tools/mod; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  FAILPOINTS=true ./build;

/tmp/etcd-v3.5.12-beforeSendWatchResponse/bin: $(GOPATH)/bin/gofail
	rm -rf /tmp/etcd-v3.5.12-beforeSendWatchResponse/
	mkdir -p /tmp/etcd-v3.5.12-beforeSendWatchResponse/
	git clone --depth 1 --branch v3.5.12 https://github.com/etcd-io/etcd.git /tmp/etcd-v3.5.12-beforeSendWatchResponse/
	cp -r ./tests/robustness/patches/beforeSendWatchResponse /tmp/etcd-v3.5.12-beforeSendWatchResponse/
	cd /tmp/etcd-v3.5.12-beforeSendWatchResponse/; \
	  patch -l server/etcdserver/api/v3rpc/watch.go ./beforeSendWatchResponse/watch.patch; \
	  patch -l build.sh ./beforeSendWatchResponse/build.patch; \
	  go get go.etcd.io/gofail@${GOFAIL_VERSION}; \
	  (cd server; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  (cd etcdctl; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  (cd etcdutl; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  (cd tools/mod; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  FAILPOINTS=true ./build;

/tmp/etcd-v3.5.13-compactBeforeSetFinishedCompact/bin: $(GOPATH)/bin/gofail
	rm -rf /tmp/etcd-v3.5.13-compactBeforeSetFinishedCompact/
	mkdir -p /tmp/etcd-v3.5.13-compactBeforeSetFinishedCompact/
	git clone --depth 1 --branch v3.5.13 https://github.com/etcd-io/etcd.git /tmp/etcd-v3.5.13-compactBeforeSetFinishedCompact/
	cp -r ./tests/robustness/patches/compactBeforeSetFinishedCompact /tmp/etcd-v3.5.13-compactBeforeSetFinishedCompact/
	cd /tmp/etcd-v3.5.13-compactBeforeSetFinishedCompact/; \
	  patch -l server/mvcc/kvstore_compaction.go ./compactBeforeSetFinishedCompact/kvstore_compaction.patch; \
	  go get go.etcd.io/gofail@${GOFAIL_VERSION}; \
	  (cd server; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  (cd etcdctl; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  (cd etcdutl; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  (cd tools/mod; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  FAILPOINTS=true ./build;

/tmp/etcd-release-3.5-failpoints/bin: $(GOPATH)/bin/gofail
	rm -rf /tmp/etcd-release-3.5-failpoints/
	mkdir -p /tmp/etcd-release-3.5-failpoints/
	cd /tmp/etcd-release-3.5-failpoints/; \
	  git clone --depth 1 --branch release-3.5 https://github.com/etcd-io/etcd.git .; \
	  go get go.etcd.io/gofail@${GOFAIL_VERSION}; \
	  (cd tools/mod; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  FAILPOINTS=true ./build;

/tmp/etcd-v3.4.23-failpoints/bin:
/tmp/etcd-v3.4.%-failpoints/bin: $(GOPATH)/bin/gofail
	rm -rf /tmp/etcd-v3.4.$*-failpoints/
	mkdir -p /tmp/etcd-v3.4.$*-failpoints/
	cd /tmp/etcd-v3.4.$*-failpoints/; \
	  git clone --depth 1 --branch v3.4.$* https://github.com/etcd-io/etcd.git .; \
	  go get go.etcd.io/gofail@${GOFAIL_VERSION}; \
	  (cd tools/mod; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  FAILPOINTS=true ./build;

/tmp/etcd-release-3.4-failpoints/bin: $(GOPATH)/bin/gofail
	rm -rf /tmp/etcd-release-3.4-failpoints/
	mkdir -p /tmp/etcd-release-3.4-failpoints/
	cd /tmp/etcd-release-3.4-failpoints/; \
	  git clone --depth 1 --branch release-3.4 https://github.com/etcd-io/etcd.git .; \
	  go get go.etcd.io/gofail@${GOFAIL_VERSION}; \
	  (cd tools/mod; go get go.etcd.io/gofail@${GOFAIL_VERSION}); \
	  FAILPOINTS=true ./build;

# run from repository root



# Example:
#   make build
#   make clean
#   make docker-clean
#   make docker-start
#   make docker-kill
#   make docker-remove

UNAME := $(shell uname)
XARGS = xargs
ARCH ?= $(shell go env GOARCH)

# -r is only necessary on GNU xargs.
ifeq ($(UNAME), Linux)
XARGS += -r
endif
XARGS += rm -r

.PHONY: build
build:
	GO_BUILD_FLAGS="-v" ./scripts/build.sh
	./bin/etcd --version
	./bin/etcdctl version
	./bin/etcdutl version

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
	find ./ -name "127.0.0.1:*" -o -name "localhost:*" -o -name "*.log" -o -name "agent-*" -o -name "*.coverprofile" -o -name "testname-proxy-*" | $(XARGS)

GO_VERSION ?= 1.17.6
ETCD_VERSION ?= $(shell git rev-parse --short HEAD || echo "GitNotFound")

TEST_SUFFIX = $(shell date +%s | base64 | head -c 15)
TEST_OPTS ?= PASSES='unit'

TMP_DIR_MOUNT_FLAG = --tmpfs=/tmp:exec
ifdef HOST_TMP_DIR
	TMP_DIR_MOUNT_FLAG = --mount type=bind,source=$(HOST_TMP_DIR),destination=/tmp
endif


TMP_DOCKERFILE:=$(shell mktemp)

# Example:
#   GO_VERSION=1.14.3 make build-docker-test
#   make build-docker-test
#
#   gcloud auth configure-docker
#   GO_VERSION=1.14.3 make push-docker-test
#   make push-docker-test
#
#   gsutil -m acl ch -u allUsers:R -r gs://artifacts.etcd-development.appspot.com
#   make pull-docker-test

build-docker-test:
	$(info GO_VERSION: $(GO_VERSION))
	@sed 's|REPLACE_ME_GO_VERSION|$(GO_VERSION)|g' ./tests/Dockerfile > $(TMP_DOCKERFILE)
	docker build \
	  --network=host \
	  --tag gcr.io/etcd-development/etcd-test:go$(GO_VERSION) \
	  --file $(TMP_DOCKERFILE) .

push-docker-test:
	$(info GO_VERSION: $(GO_VERSION))
	docker push gcr.io/etcd-development/etcd-test:go$(GO_VERSION)

pull-docker-test:
	$(info GO_VERSION: $(GO_VERSION))
	docker pull gcr.io/etcd-development/etcd-test:go$(GO_VERSION)

# Example:
#
# Local machine:
#   TEST_OPTS="PASSES='fmt'" make test
#   TEST_OPTS="PASSES='fmt bom dep build unit'" make test
#   TEST_OPTS="PASSES='build unit release integration_e2e functional'" make test
#   TEST_OPTS="PASSES='build grpcproxy'" make test
#
# grpc-proxy tests:
#   TEST_OPTS="PASSES='build grpcproxy'" make test
#   HOST_TMP_DIR=/tmp TEST_OPTS="PASSES='build grpcproxy'" make test

.PHONY: test
test:
	$(info TEST_OPTS: $(TEST_OPTS))
	$(info log-file: test-$(TEST_SUFFIX).log)
	$(TEST_OPTS) ./scripts/test.sh 2>&1 | tee test-$(TEST_SUFFIX).log
	! egrep "(--- FAIL:|FAIL:|DATA RACE|panic: test timed out|appears to have leaked)" -B50 -A10 test-$(TEST_SUFFIX).log

test-smoke:
	$(info log-file: test-$(TEST_SUFFIX).log)
	PASSES="fmt build unit" ./scripts/test.sh 2<&1 | tee test-$(TEST_SUFFIX).log

test-full:
	$(info log-file: test-$(TEST_SUFFIX).log)
	PASSES="fmt build release unit integration functional e2e grpcproxy" ./scripts/test.sh 2<&1 | tee test-$(TEST_SUFFIX).log

ensure-docker-test-image-exists:
	make push-docker-test || echo "WARNING: Container Image not found in registry, building locally"; make build-docker-test

docker-test: ensure-docker-test-image-exists
	$(info GO_VERSION: $(GO_VERSION))
	$(info ETCD_VERSION: $(ETCD_VERSION))
	$(info TEST_OPTS: $(TEST_OPTS))
	$(info log-file: test-$(TEST_SUFFIX).log)
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`,destination=/go/src/go.etcd.io/etcd \
	  gcr.io/etcd-development/etcd-test:go$(GO_VERSION) \
	  /bin/bash -c "$(TEST_OPTS) ./scripts/test.sh 2>&1 | tee test-$(TEST_SUFFIX).log"
	! egrep "(--- FAIL:|FAIL:|DATA RACE|panic: test timed out|appears to have leaked)" -B50 -A10 test-$(TEST_SUFFIX).log

docker-test-coverage:
	$(info GO_VERSION: $(GO_VERSION))
	$(info ETCD_VERSION: $(ETCD_VERSION))
	$(info log-file: docker-test-coverage-$(TEST_SUFFIX).log)
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`,destination=/go/src/go.etcd.io/etcd \
	  gcr.io/etcd-development/etcd-test:go$(GO_VERSION) \
	  /bin/bash ./scripts/codecov_upload.sh docker-test-coverage-$(TEST_SUFFIX).log \
	! egrep "(--- FAIL:|FAIL:|DATA RACE|panic: test timed out|appears to have leaked)" -B50 -A10 docker-test-coverage-$(TEST_SUFFIX).log


# Example:
#   ETCD_VERSION=v3-test make build-docker-release-main
#   ETCD_VERSION=v3-test make push-docker-release-main
#   gsutil -m acl ch -u allUsers:R -r gs://artifacts.etcd-development.appspot.com

build-docker-release-main:
	$(info ETCD_VERSION: $(ETCD_VERSION))
	cp ./Dockerfile-release.$(ARCH) ./bin/Dockerfile-release.$(ARCH)
	docker build \
	  --network=host \
	  --tag gcr.io/etcd-development/etcd:$(ETCD_VERSION) \
	  --file ./bin/Dockerfile-release.$(ARCH) \
	  ./bin
	rm -f ./bin/Dockerfile-release.$(ARCH)

	docker run \
	  --rm \
	  gcr.io/etcd-development/etcd:$(ETCD_VERSION) \
	  /bin/sh -c "/usr/local/bin/etcd --version && /usr/local/bin/etcdctl version && /usr/local/bin/etcdutl version"

push-docker-release-main:
	$(info ETCD_VERSION: $(ETCD_VERSION))
	docker push gcr.io/etcd-development/etcd:$(ETCD_VERSION)

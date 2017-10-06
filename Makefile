# run from repository root

TEST_SUFFIX = $(shell date +%s | base64 | head -c 15)

.PHONY: build
build:
	GO_BUILD_FLAGS="-v" ./build
	./bin/etcd --version
	ETCDCTL_API=3 ./bin/etcdctl version

test:
	$(info log-file: test-$(TEST_SUFFIX).log)
	PASSES='fmt bom dep compile build unit' ./test 2>&1 | tee test-$(TEST_SUFFIX).log
	! grep FAIL -A10 -B50 test-$(TEST_SUFFIX).log

test-all:
	$(info log-file: test-all-$(TEST_SUFFIX).log)
	RELEASE_TEST=y INTEGRATION=y PASSES='build unit release integration_e2e functional' ./test 2>&1 | tee test-all-$(TEST_SUFFIX).log
	! grep FAIL -A10 -B50 test-all-$(TEST_SUFFIX).log

test-proxy:
	$(info log-file: test-proxy-$(TEST_SUFFIX).log)
	PASSES='build grpcproxy' ./test 2>&1 | tee test-proxy-$(TEST_SUFFIX).log
	! grep FAIL -A10 -B50 test-proxy-$(TEST_SUFFIX).log

test-coverage:
	$(info log-file: test-coverage-$(TEST_SUFFIX).log)
	COVERDIR=covdir PASSES='build build_cov cov' ./test 2>&1 | tee test-coverage-$(TEST_SUFFIX).log
	$(shell curl -s https://codecov.io/bash >codecov)
	chmod 700 ./codecov
	./codecov -h
	./codecov -t 6040de41-c073-4d6f-bbf8-d89256ef31e1

# clean up failed tests, logs, dependencies
clean:
	rm -f ./codecov
	rm -f ./*.log
	rm -f ./bin/Dockerfile-release
	rm -rf ./bin/*.etcd
	rm -rf ./gopath
	rm -rf ./release
	rm -f ./integration/127.0.0.1:* ./integration/localhost:*
	rm -f ./clientv3/integration/127.0.0.1:* ./clientv3/integration/localhost:*
	rm -f ./clientv3/ordering/127.0.0.1:* ./clientv3/ordering/localhost:*

# sync with Dockerfile-test, e2e/docker-dns/Dockerfile, e2e/docker-dns-srv/Dockerfile
_GO_VERSION = go1.8.4
ifdef GO_VERSION
	_GO_VERSION = $(GO_VERSION)
endif

# build base container image for testing on Linux
docker-test-build:
	docker build --tag gcr.io/etcd-development/etcd-test:$(_GO_VERSION) --file ./Dockerfile-test .

# e.g.
# gcloud docker -- login -u _json_key -p "$(cat /etc/gcp-key-etcd.json)" https://gcr.io
docker-test-push:
	gcloud docker -- push gcr.io/etcd-development/etcd-test:$(_GO_VERSION)

docker-test-pull:
	docker pull gcr.io/etcd-development/etcd-test:$(_GO_VERSION)

# compile etcd and etcdctl with Linux
docker-test-compile:
	docker run \
	  --rm \
	  --volume=`pwd`/:/etcd \
	  gcr.io/etcd-development/etcd-test:$(_GO_VERSION) \
	  /bin/bash -c "cd /etcd && GO_BUILD_FLAGS=-v ./build && ./bin/etcd --version"

# run tests inside container
docker-test:
	$(info log-file: docker-test-$(TEST_SUFFIX).log)
	docker run \
	  --rm \
	  --volume=`pwd`:/go/src/github.com/coreos/etcd \
	  gcr.io/etcd-development/etcd-test:$(_GO_VERSION) \
	  /bin/bash -c "RELEASE_TEST=y INTEGRATION=y PASSES='build unit release integration_e2e functional' ./test 2>&1 | tee docker-test-$(TEST_SUFFIX).log"
	! grep FAIL -A10 -B50 docker-test-$(TEST_SUFFIX).log

docker-test-386:
	$(info log-file: docker-test-386-$(TEST_SUFFIX).log)
	docker run \
	  --rm \
	  --volume=`pwd`:/go/src/github.com/coreos/etcd \
	  gcr.io/etcd-development/etcd-test:$(_GO_VERSION) \
	  /bin/bash -c "GOARCH=386 PASSES='build unit integration_e2e' ./test 2>&1 | tee docker-test-386-$(TEST_SUFFIX).log"
	! grep FAIL -A10 -B50 docker-test-386-$(TEST_SUFFIX).log

docker-test-proxy:
	$(info log-file: docker-test-proxy-$(TEST_SUFFIX).log)
	docker run \
	  --rm \
	  --volume=`pwd`:/go/src/github.com/coreos/etcd \
	  gcr.io/etcd-development/etcd-test:$(_GO_VERSION) \
	  /bin/bash -c "PASSES='build grpcproxy' ./test ./test 2>&1 | tee docker-test-proxy-$(TEST_SUFFIX).log"
	! grep FAIL -A10 -B50 docker-test-proxy-$(TEST_SUFFIX).log

# build release container image with Linux
_ETCD_VERSION ?= $(shell git rev-parse --short HEAD || echo "GitNotFound")
ifdef ETCD_VERSION
	_ETCD_VERSION = $(ETCD_VERSION)
endif

docker-release-master-build: docker-test-compile
	cp ./Dockerfile-release ./bin/Dockerfile-release
	docker build \
	  --tag gcr.io/etcd-development/etcd:$(_ETCD_VERSION) \
	  --file ./bin/Dockerfile-release \
	  ./bin
	rm -f ./bin/Dockerfile-release

	docker run \
	  --rm \
	  gcr.io/etcd-development/etcd:$(_ETCD_VERSION) \
	  /bin/sh -c "/usr/local/bin/etcd --version && ETCDCTL_API=3 /usr/local/bin/etcdctl version"

docker-release-master-push:
	gcloud docker -- push gcr.io/etcd-development/etcd:$(_ETCD_VERSION)

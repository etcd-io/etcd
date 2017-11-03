# run from repository root
#
# Example:
#   make clean -f ./hack/scripts-dev/Makefile
#   make build -f ./hack/scripts-dev/Makefile

.PHONY: build
build:
	GO_BUILD_FLAGS="-v" ./build
	./bin/etcd --version
	ETCDCTL_API=3 ./bin/etcdctl version

clean:
	rm -f ./codecov
	rm -rf ./covdir
	rm -f ./*.log
	rm -f ./bin/Dockerfile-release
	rm -rf ./bin/*.etcd
	rm -rf ./gopath
	rm -rf ./release
	rm -f ./integration/127.0.0.1:* ./integration/localhost:*
	rm -f ./clientv3/integration/127.0.0.1:* ./clientv3/integration/localhost:*
	rm -f ./clientv3/ordering/127.0.0.1:* ./clientv3/ordering/localhost:*

_GO_VERSION = 1.9.2
ifdef GO_VERSION
	_GO_VERSION = $(GO_VERSION)
endif

# Example:
#   GO_VERSION=1.8.5 make build-docker-test -f ./hack/scripts-dev/Makefile
#   make build-docker-test -f ./hack/scripts-dev/Makefile
#   gcloud docker -- login -u _json_key -p "$(cat /etc/gcp-key-etcd.json)" https://gcr.io
#   GO_VERSION=1.8.5 make push-docker-test -f ./hack/scripts-dev/Makefile
#   make push-docker-test -f ./hack/scripts-dev/Makefile
#   gsutil -m acl ch -u allUsers:R -r gs://artifacts.etcd-development.appspot.com
#   GO_VERSION=1.8.5 make pull-docker-test -f ./hack/scripts-dev/Makefile
#   make pull-docker-test -f ./hack/scripts-dev/Makefile

build-docker-test:
	$(info GO_VERSION: $(_GO_VERSION))
	@cat ./Dockerfile-test | sed s/REPLACE_ME_GO_VERSION/$(_GO_VERSION)/ \
	  > ./.Dockerfile-test
	docker build \
	  --tag gcr.io/etcd-development/etcd-test:go$(_GO_VERSION) \
	  --file ./.Dockerfile-test .

push-docker-test:
	$(info GO_VERSION: $(_GO_VERSION))
	gcloud docker -- push gcr.io/etcd-development/etcd-test:go$(_GO_VERSION)

pull-docker-test:
	$(info GO_VERSION: $(_GO_VERSION))
	docker pull gcr.io/etcd-development/etcd-test:go$(_GO_VERSION)

compile-with-docker-test:
	$(info GO_VERSION: $(_GO_VERSION))
	docker run \
	  --rm \
	  --volume=`pwd`/:/etcd \
	  gcr.io/etcd-development/etcd-test:go$(_GO_VERSION) \
	  /bin/bash -c "cd /etcd && GO_BUILD_FLAGS=-v ./build && ./bin/etcd --version"

# Local machine:
#   TEST_OPTS="PASSES='fmt'" make test -f ./hack/scripts-dev/Makefile
#   TEST_OPTS="PASSES='fmt bom dep compile build unit'" make test -f ./hack/scripts-dev/Makefile
#   TEST_OPTS="RELEASE_TEST=y INTEGRATION=y PASSES='build unit release integration_e2e functional'" make test -f ./hack/scripts-dev/Makefile
#   TEST_OPTS="PASSES='build grpcproxy'" make test -f ./hack/scripts-dev/Makefile
#
# Example (test with docker):
#   make pull-docker-test -f ./hack/scripts-dev/Makefile
#   TEST_OPTS="PASSES='fmt'" make docker-test -f ./hack/scripts-dev/Makefile
#   TEST_OPTS="VERBOSE=2 PASSES='unit'" make docker-test -f ./hack/scripts-dev/Makefile
#
# Travis CI (test with docker):
#   TEST_OPTS="PASSES='fmt bom dep compile build unit'" make docker-test -f ./hack/scripts-dev/Makefile
#
# Semaphore CI (test with docker):
#   TEST_OPTS="RELEASE_TEST=y INTEGRATION=y PASSES='build unit release integration_e2e functional'" make docker-test -f ./hack/scripts-dev/Makefile
#   TEST_OPTS="GOARCH=386 PASSES='build unit integration_e2e'" make docker-test -f ./hack/scripts-dev/Makefile
#
# grpc-proxy tests (test with docker):
#   TEST_OPTS="PASSES='build grpcproxy'" make docker-test -f ./hack/scripts-dev/Makefile

TEST_SUFFIX = $(shell date +%s | base64 | head -c 15)

_TEST_OPTS = "PASSES='unit'"
ifdef TEST_OPTS
	_TEST_OPTS = $(TEST_OPTS)
endif

.PHONY: test
test:
	$(info TEST_OPTS: $(_TEST_OPTS))
	$(info log-file: test-$(TEST_SUFFIX).log)
	$(_TEST_OPTS) ./test 2>&1 | tee test-$(TEST_SUFFIX).log
	! egrep "(--- FAIL:|leak)" -A10 -B50 test-$(TEST_SUFFIX).log

docker-test:
	$(info GO_VERSION: $(_GO_VERSION))
	$(info TEST_OPTS: $(_TEST_OPTS))
	$(info log-file: test-$(TEST_SUFFIX).log)
	docker run \
	  --rm \
	  --volume=`pwd`:/go/src/github.com/coreos/etcd \
	  gcr.io/etcd-development/etcd-test:go$(_GO_VERSION) \
	  /bin/bash -c "$(_TEST_OPTS) ./test 2>&1 | tee test-$(TEST_SUFFIX).log"
	! egrep "(--- FAIL:|leak)" -A10 -B50 test-$(TEST_SUFFIX).log

docker-test-coverage:
	$(info GO_VERSION: $(_GO_VERSION))
	$(info log-file: docker-test-coverage-$(TEST_SUFFIX).log)
	docker run \
	  --rm \
	  --volume=`pwd`:/go/src/github.com/coreos/etcd \
	  gcr.io/etcd-development/etcd-test:go$(_GO_VERSION) \
	  /bin/bash -c "COVERDIR=covdir PASSES='build build_cov cov' ./test 2>&1 | tee docker-test-coverage-$(TEST_SUFFIX).log && /codecov -t 6040de41-c073-4d6f-bbf8-d89256ef31e1"
	! egrep "(--- FAIL:|leak)" -A10 -B50 docker-test-coverage-$(TEST_SUFFIX).log

# build release container image with Linux
_ETCD_VERSION ?= $(shell git rev-parse --short HEAD || echo "GitNotFound")
ifdef ETCD_VERSION
	_ETCD_VERSION = $(ETCD_VERSION)
endif

# Example:
#   ETCD_VERSION=v3.3.0-test.0 make build-docker-release-master -f ./hack/scripts-dev/Makefile
#   ETCD_VERSION=v3.3.0-test.0 make push-docker-release-master -f ./hack/scripts-dev/Makefile
#   gsutil -m acl ch -u allUsers:R -r gs://artifacts.etcd-development.appspot.com

build-docker-release-master: compile-with-docker-test
	$(info ETCD_VERSION: $(_ETCD_VERSION))
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

push-docker-release-master:
	$(info ETCD_VERSION: $(_ETCD_VERSION))
	gcloud docker -- push gcr.io/etcd-development/etcd:$(_ETCD_VERSION)

# Example:
#   make build-docker-test -f ./hack/scripts-dev/Makefile
#   make compile-with-docker-test -f ./hack/scripts-dev/Makefile
#   make build-docker-dns-test -f ./hack/scripts-dev/Makefile
#   gcloud docker -- login -u _json_key -p "$(cat /etc/gcp-key-etcd.json)" https://gcr.io
#   make push-docker-dns-test -f ./hack/scripts-dev/Makefile
#   gsutil -m acl ch -u allUsers:R -r gs://artifacts.etcd-development.appspot.com
#   make pull-docker-dns-test -f ./hack/scripts-dev/Makefile
#   make docker-dns-test-run -f ./hack/scripts-dev/Makefile

# build base container image for DNS testing
build-docker-dns-test:
	$(info GO_VERSION: $(_GO_VERSION))
	@cat ./hack/scripts-dev/docker-dns/Dockerfile | sed s/REPLACE_ME_GO_VERSION/$(_GO_VERSION)/ \
	  > ./hack/scripts-dev/docker-dns/.Dockerfile

	docker build \
	  --tag gcr.io/etcd-development/etcd-dns-test:go$(_GO_VERSION) \
	  --file ./hack/scripts-dev/docker-dns/.Dockerfile \
	  ./hack/scripts-dev/docker-dns

	docker run \
	  --rm \
	  --dns 127.0.0.1 \
	  gcr.io/etcd-development/etcd-dns-test:go$(_GO_VERSION) \
	  /bin/bash -c "/etc/init.d/bind9 start && cat /dev/null >/etc/hosts && dig etcd.local"

push-docker-dns-test:
	$(info GO_VERSION: $(_GO_VERSION))
	gcloud docker -- push gcr.io/etcd-development/etcd-dns-test:go$(_GO_VERSION)

pull-docker-dns-test:
	$(info GO_VERSION: $(_GO_VERSION))
	docker pull gcr.io/etcd-development/etcd-dns-test:go$(_GO_VERSION)

# run DNS tests inside container
docker-dns-test-run:
	$(info GO_VERSION: $(_GO_VERSION))
	docker run \
	  --rm \
	  --tty \
	  --dns 127.0.0.1 \
	  --volume=`pwd`/bin:/etcd \
	  --volume=`pwd`/integration/fixtures:/certs \
	  gcr.io/etcd-development/etcd-dns-test:go$(_GO_VERSION) \
	  /bin/bash -c "cd /etcd && /run.sh && rm -rf m*.etcd"

# Example:
#   make build-docker-test -f ./hack/scripts-dev/Makefile
#   make compile-with-docker-test -f ./hack/scripts-dev/Makefile
#   make build-docker-dns-srv-test -f ./hack/scripts-dev/Makefile
#   gcloud docker -- login -u _json_key -p "$(cat /etc/gcp-key-etcd.json)" https://gcr.io
#   make push-docker-dns-srv-test -f ./hack/scripts-dev/Makefile
#   gsutil -m acl ch -u allUsers:R -r gs://artifacts.etcd-development.appspot.com
#   make pull-docker-dns-srv-test -f ./hack/scripts-dev/Makefile
#   make docker-dns-srv-test-run -f ./hack/scripts-dev/Makefile

# build base container image for DNS/SRV testing
build-docker-dns-srv-test:
	$(info GO_VERSION: $(_GO_VERSION))
	@cat ./hack/scripts-dev/docker-dns-srv/Dockerfile | sed s/REPLACE_ME_GO_VERSION/$(_GO_VERSION)/ \
	  > ./hack/scripts-dev/docker-dns-srv/.Dockerfile

	docker build \
	  --tag gcr.io/etcd-development/etcd-dns-srv-test:go$(_GO_VERSION) \
	  --file ./hack/scripts-dev/docker-dns-srv/.Dockerfile \
	  ./hack/scripts-dev/docker-dns-srv

	docker run \
	  --rm \
	  --dns 127.0.0.1 \
	  gcr.io/etcd-development/etcd-dns-srv-test:go$(_GO_VERSION) \
	  /bin/bash -c "/etc/init.d/bind9 start && cat /dev/null >/etc/hosts && dig +noall +answer SRV _etcd-client-ssl._tcp.etcd.local && dig +noall +answer SRV _etcd-server-ssl._tcp.etcd.local && dig +noall +answer m1.etcd.local m2.etcd.local m3.etcd.local"

push-docker-dns-srv-test:
	$(info GO_VERSION: $(_GO_VERSION))
	gcloud docker -- push gcr.io/etcd-development/etcd-dns-srv-test:go$(_GO_VERSION)

pull-docker-dns-srv-test:
	$(info GO_VERSION: $(_GO_VERSION))
	docker pull gcr.io/etcd-development/etcd-dns-srv-test:go$(_GO_VERSION)

# run DNS/SRV tests inside container
docker-dns-srv-test-run:
	$(info GO_VERSION: $(_GO_VERSION))
	docker run \
	  --rm \
	  --tty \
	  --dns 127.0.0.1 \
	  --volume=`pwd`/bin:/etcd \
	  --volume=`pwd`/integration/fixtures:/certs \
	  gcr.io/etcd-development/etcd-dns-srv-test:go$(_GO_VERSION) \
	  /bin/bash -c "cd /etcd && /run.sh && rm -rf m*.etcd"

# TODO: add DNS integration tests

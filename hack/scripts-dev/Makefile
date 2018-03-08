# run from repository root



# Example:
#   make build -f ./hack/scripts-dev/Makefile
#   make clean -f ./hack/scripts-dev/Makefile
#   make clean-docker -f ./hack/scripts-dev/Makefile
#   make restart-docker -f ./hack/scripts-dev/Makefile
#   make delete-docker-images -f ./hack/scripts-dev/Makefile

.PHONY: build
build:
	GO_BUILD_FLAGS="-v" ./build
	./bin/etcd --version
	ETCDCTL_API=3 ./bin/etcdctl version

clean:
	rm -f ./codecov
	rm -rf ./agent-*
	rm -rf ./covdir
	rm -f ./*.log
	rm -f ./bin/Dockerfile-release
	rm -rf ./bin/*.etcd
	rm -rf ./gopath
	rm -rf ./release
	rm -f ./integration/127.0.0.1:* ./integration/localhost:*
	rm -f ./clientv3/integration/127.0.0.1:* ./clientv3/integration/localhost:*
	rm -f ./clientv3/ordering/127.0.0.1:* ./clientv3/ordering/localhost:*

clean-docker:
	docker images
	docker image prune --force

restart-docker:
	service docker restart

delete-docker-images:
	docker rm --force $(docker ps -a -q) || true
	docker rmi --force $(docker images -q) || true



GO_VERSION ?= 1.10
ETCD_VERSION ?= $(shell git rev-parse --short HEAD || echo "GitNotFound")

TEST_SUFFIX = $(shell date +%s | base64 | head -c 15)
TEST_OPTS ?= PASSES='unit'

TMP_DIR_MOUNT_FLAG = --mount type=tmpfs,destination=/tmp
ifdef HOST_TMP_DIR
	TMP_DIR_MOUNT_FLAG = --mount type=bind,source=$(HOST_TMP_DIR),destination=/tmp
endif



# Example:
#   GO_VERSION=1.8.7 make build-docker-test -f ./hack/scripts-dev/Makefile
#   make build-docker-test -f ./hack/scripts-dev/Makefile
#   gcloud docker -- login -u _json_key -p "$(cat /etc/gcp-key-etcd-development.json)" https://gcr.io
#   GO_VERSION=1.8.7 make push-docker-test -f ./hack/scripts-dev/Makefile
#   make push-docker-test -f ./hack/scripts-dev/Makefile
#   gsutil -m acl ch -u allUsers:R -r gs://artifacts.etcd-development.appspot.com
#   GO_VERSION=1.8.7 make pull-docker-test -f ./hack/scripts-dev/Makefile
#   make pull-docker-test -f ./hack/scripts-dev/Makefile

build-docker-test:
	$(info GO_VERSION: $(GO_VERSION))
	@sed -i.bak 's|REPLACE_ME_GO_VERSION|$(GO_VERSION)|g' ./Dockerfile-test
	docker build \
	  --tag gcr.io/etcd-development/etcd-test:go$(GO_VERSION) \
	  --file ./Dockerfile-test .
	@mv ./Dockerfile-test.bak ./Dockerfile-test

push-docker-test:
	$(info GO_VERSION: $(GO_VERSION))
	gcloud docker -- push gcr.io/etcd-development/etcd-test:go$(GO_VERSION)

pull-docker-test:
	$(info GO_VERSION: $(GO_VERSION))
	docker pull gcr.io/etcd-development/etcd-test:go$(GO_VERSION)



# Example:
#   make build-docker-test -f ./hack/scripts-dev/Makefile
#   make compile-with-docker-test -f ./hack/scripts-dev/Makefile
#   make compile-setup-gopath-with-docker-test -f ./hack/scripts-dev/Makefile

compile-with-docker-test:
	$(info GO_VERSION: $(GO_VERSION))
	docker run \
	  --rm \
	  --mount type=bind,source=`pwd`,destination=/go/src/github.com/coreos/etcd \
	  gcr.io/etcd-development/etcd-test:go$(GO_VERSION) \
	  /bin/bash -c "GO_BUILD_FLAGS=-v ./build && ./bin/etcd --version"

compile-setup-gopath-with-docker-test:
	$(info GO_VERSION: $(GO_VERSION))
	docker run \
	  --rm \
	  --mount type=bind,source=`pwd`,destination=/etcd \
	  gcr.io/etcd-development/etcd-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && ETCD_SETUP_GOPATH=1 GO_BUILD_FLAGS=-v ./build && ./bin/etcd --version && rm -rf ./gopath"



# Example:
#
# Local machine:
#   TEST_OPTS="PASSES='fmt'" make test -f ./hack/scripts-dev/Makefile
#   TEST_OPTS="PASSES='fmt bom dep compile build unit'" make test -f ./hack/scripts-dev/Makefile
#   TEST_OPTS="PASSES='build unit release integration_e2e functional'" make test -f ./hack/scripts-dev/Makefile
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
#   TEST_OPTS="PASSES='build unit release integration_e2e functional'" make docker-test -f ./hack/scripts-dev/Makefile
#   HOST_TMP_DIR=/tmp TEST_OPTS="PASSES='build unit release integration_e2e functional'" make docker-test -f ./hack/scripts-dev/Makefile
#   TEST_OPTS="GOARCH=386 PASSES='build unit integration_e2e'" make docker-test -f ./hack/scripts-dev/Makefile
#
# grpc-proxy tests (test with docker):
#   TEST_OPTS="PASSES='build grpcproxy'" make docker-test -f ./hack/scripts-dev/Makefile
#   HOST_TMP_DIR=/tmp TEST_OPTS="PASSES='build grpcproxy'" make docker-test -f ./hack/scripts-dev/Makefile

.PHONY: test
test:
	$(info TEST_OPTS: $(TEST_OPTS))
	$(info log-file: test-$(TEST_SUFFIX).log)
	$(TEST_OPTS) ./test 2>&1 | tee test-$(TEST_SUFFIX).log
	! egrep "(--- FAIL:|panic: test timed out|appears to have leaked)" -B50 -A10 test-$(TEST_SUFFIX).log

docker-test:
	$(info GO_VERSION: $(GO_VERSION))
	$(info ETCD_VERSION: $(ETCD_VERSION))
	$(info TEST_OPTS: $(TEST_OPTS))
	$(info log-file: test-$(TEST_SUFFIX).log)
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`,destination=/go/src/github.com/coreos/etcd \
	  gcr.io/etcd-development/etcd-test:go$(GO_VERSION) \
	  /bin/bash -c "$(TEST_OPTS) ./test 2>&1 | tee test-$(TEST_SUFFIX).log"
	! egrep "(--- FAIL:|panic: test timed out|appears to have leaked)" -B50 -A10 test-$(TEST_SUFFIX).log

docker-test-coverage:
	$(info GO_VERSION: $(GO_VERSION))
	$(info ETCD_VERSION: $(ETCD_VERSION))
	$(info log-file: docker-test-coverage-$(TEST_SUFFIX).log)
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`,destination=/go/src/github.com/coreos/etcd \
	  gcr.io/etcd-development/etcd-test:go$(GO_VERSION) \
	  /bin/bash -c "COVERDIR=covdir PASSES='build build_cov cov' ./test 2>&1 | tee docker-test-coverage-$(TEST_SUFFIX).log && /codecov -t 6040de41-c073-4d6f-bbf8-d89256ef31e1"
	! egrep "(--- FAIL:|panic: test timed out|appears to have leaked)" -B50 -A10 docker-test-coverage-$(TEST_SUFFIX).log



# Example:
#   make compile-with-docker-test -f ./hack/scripts-dev/Makefile
#   ETCD_VERSION=v3-test make build-docker-release-master -f ./hack/scripts-dev/Makefile
#   ETCD_VERSION=v3-test make push-docker-release-master -f ./hack/scripts-dev/Makefile
#   gsutil -m acl ch -u allUsers:R -r gs://artifacts.etcd-development.appspot.com

build-docker-release-master:
	$(info ETCD_VERSION: $(ETCD_VERSION))
	cp ./Dockerfile-release ./bin/Dockerfile-release
	docker build \
	  --tag gcr.io/etcd-development/etcd:$(ETCD_VERSION) \
	  --file ./bin/Dockerfile-release \
	  ./bin
	rm -f ./bin/Dockerfile-release

	docker run \
	  --rm \
	  gcr.io/etcd-development/etcd:$(ETCD_VERSION) \
	  /bin/sh -c "/usr/local/bin/etcd --version && ETCDCTL_API=3 /usr/local/bin/etcdctl version"

push-docker-release-master:
	$(info ETCD_VERSION: $(ETCD_VERSION))
	gcloud docker -- push gcr.io/etcd-development/etcd:$(ETCD_VERSION)



# Example:
#   make build-docker-test -f ./hack/scripts-dev/Makefile
#   make compile-with-docker-test -f ./hack/scripts-dev/Makefile
#   make build-docker-static-ip-test -f ./hack/scripts-dev/Makefile
#   gcloud docker -- login -u _json_key -p "$(cat /etc/gcp-key-etcd-development.json)" https://gcr.io
#   make push-docker-static-ip-test -f ./hack/scripts-dev/Makefile
#   gsutil -m acl ch -u allUsers:R -r gs://artifacts.etcd-development.appspot.com
#   make pull-docker-static-ip-test -f ./hack/scripts-dev/Makefile
#   make docker-static-ip-test-certs-run -f ./hack/scripts-dev/Makefile
#   make docker-static-ip-test-certs-metrics-proxy-run -f ./hack/scripts-dev/Makefile

build-docker-static-ip-test:
	$(info GO_VERSION: $(GO_VERSION))
	@sed -i.bak 's|REPLACE_ME_GO_VERSION|$(GO_VERSION)|g' ./hack/scripts-dev/docker-static-ip/Dockerfile
	docker build \
	  --tag gcr.io/etcd-development/etcd-static-ip-test:go$(GO_VERSION) \
	  --file ./hack/scripts-dev/docker-static-ip/Dockerfile \
	  ./hack/scripts-dev/docker-static-ip
	@mv ./hack/scripts-dev/docker-static-ip/Dockerfile.bak ./hack/scripts-dev/docker-static-ip/Dockerfile

push-docker-static-ip-test:
	$(info GO_VERSION: $(GO_VERSION))
	gcloud docker -- push gcr.io/etcd-development/etcd-static-ip-test:go$(GO_VERSION)

pull-docker-static-ip-test:
	$(info GO_VERSION: $(GO_VERSION))
	docker pull gcr.io/etcd-development/etcd-static-ip-test:go$(GO_VERSION)

docker-static-ip-test-certs-run:
	$(info GO_VERSION: $(GO_VERSION))
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  --tty \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`/bin,destination=/etcd \
	  --mount type=bind,source=`pwd`/hack/scripts-dev/docker-static-ip/certs,destination=/certs \
	  gcr.io/etcd-development/etcd-static-ip-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /certs/run.sh && rm -rf m*.etcd"

docker-static-ip-test-certs-metrics-proxy-run:
	$(info GO_VERSION: $(GO_VERSION))
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  --tty \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`/bin,destination=/etcd \
	  --mount type=bind,source=`pwd`/hack/scripts-dev/docker-static-ip/certs-metrics-proxy,destination=/certs-metrics-proxy \
	  gcr.io/etcd-development/etcd-static-ip-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /certs-metrics-proxy/run.sh && rm -rf m*.etcd"



# Example:
#   make build-docker-test -f ./hack/scripts-dev/Makefile
#   make compile-with-docker-test -f ./hack/scripts-dev/Makefile
#   make build-docker-dns-test -f ./hack/scripts-dev/Makefile
#   gcloud docker -- login -u _json_key -p "$(cat /etc/gcp-key-etcd-development.json)" https://gcr.io
#   make push-docker-dns-test -f ./hack/scripts-dev/Makefile
#   gsutil -m acl ch -u allUsers:R -r gs://artifacts.etcd-development.appspot.com
#   make pull-docker-dns-test -f ./hack/scripts-dev/Makefile
#   make docker-dns-test-insecure-run -f ./hack/scripts-dev/Makefile
#   make docker-dns-test-certs-run -f ./hack/scripts-dev/Makefile
#   make docker-dns-test-certs-gateway-run -f ./hack/scripts-dev/Makefile
#   make docker-dns-test-certs-wildcard-run -f ./hack/scripts-dev/Makefile
#   make docker-dns-test-certs-common-name-auth-run -f ./hack/scripts-dev/Makefile
#   make docker-dns-test-certs-common-name-multi-run -f ./hack/scripts-dev/Makefile

build-docker-dns-test:
	$(info GO_VERSION: $(GO_VERSION))
	@sed -i.bak 's|REPLACE_ME_GO_VERSION|$(GO_VERSION)|g' ./hack/scripts-dev/docker-dns/Dockerfile
	docker build \
	  --tag gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION) \
	  --file ./hack/scripts-dev/docker-dns/Dockerfile \
	  ./hack/scripts-dev/docker-dns
	@mv ./hack/scripts-dev/docker-dns/Dockerfile.bak ./hack/scripts-dev/docker-dns/Dockerfile

	docker run \
	  --rm \
	  --dns 127.0.0.1 \
	  gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION) \
	  /bin/bash -c "/etc/init.d/bind9 start && cat /dev/null >/etc/hosts && dig etcd.local"

push-docker-dns-test:
	$(info GO_VERSION: $(GO_VERSION))
	gcloud docker -- push gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION)

pull-docker-dns-test:
	$(info GO_VERSION: $(GO_VERSION))
	docker pull gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION)

docker-dns-test-insecure-run:
	$(info GO_VERSION: $(GO_VERSION))
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  --tty \
	  --dns 127.0.0.1 \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`/bin,destination=/etcd \
	  --mount type=bind,source=`pwd`/hack/scripts-dev/docker-dns/insecure,destination=/insecure \
	  gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /insecure/run.sh && rm -rf m*.etcd"

docker-dns-test-certs-run:
	$(info GO_VERSION: $(GO_VERSION))
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  --tty \
	  --dns 127.0.0.1 \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`/bin,destination=/etcd \
	  --mount type=bind,source=`pwd`/hack/scripts-dev/docker-dns/certs,destination=/certs \
	  gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /certs/run.sh && rm -rf m*.etcd"

docker-dns-test-certs-gateway-run:
	$(info GO_VERSION: $(GO_VERSION))
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  --tty \
	  --dns 127.0.0.1 \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`/bin,destination=/etcd \
	  --mount type=bind,source=`pwd`/hack/scripts-dev/docker-dns/certs-gateway,destination=/certs-gateway \
	  gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /certs-gateway/run.sh && rm -rf m*.etcd"

docker-dns-test-certs-wildcard-run:
	$(info GO_VERSION: $(GO_VERSION))
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  --tty \
	  --dns 127.0.0.1 \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`/bin,destination=/etcd \
	  --mount type=bind,source=`pwd`/hack/scripts-dev/docker-dns/certs-wildcard,destination=/certs-wildcard \
	  gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /certs-wildcard/run.sh && rm -rf m*.etcd"

docker-dns-test-certs-common-name-auth-run:
	$(info GO_VERSION: $(GO_VERSION))
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  --tty \
	  --dns 127.0.0.1 \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`/bin,destination=/etcd \
	  --mount type=bind,source=`pwd`/hack/scripts-dev/docker-dns/certs-common-name-auth,destination=/certs-common-name-auth \
	  gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /certs-common-name-auth/run.sh && rm -rf m*.etcd"

docker-dns-test-certs-common-name-multi-run:
	$(info GO_VERSION: $(GO_VERSION))
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  --tty \
	  --dns 127.0.0.1 \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`/bin,destination=/etcd \
	  --mount type=bind,source=`pwd`/hack/scripts-dev/docker-dns/certs-common-name-multi,destination=/certs-common-name-multi \
	  gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /certs-common-name-multi/run.sh && rm -rf m*.etcd"



# Example:
#   make build-docker-test -f ./hack/scripts-dev/Makefile
#   make compile-with-docker-test -f ./hack/scripts-dev/Makefile
#   make build-docker-dns-srv-test -f ./hack/scripts-dev/Makefile
#   gcloud docker -- login -u _json_key -p "$(cat /etc/gcp-key-etcd-development.json)" https://gcr.io
#   make push-docker-dns-srv-test -f ./hack/scripts-dev/Makefile
#   gsutil -m acl ch -u allUsers:R -r gs://artifacts.etcd-development.appspot.com
#   make pull-docker-dns-srv-test -f ./hack/scripts-dev/Makefile
#   make docker-dns-srv-test-certs-run -f ./hack/scripts-dev/Makefile
#   make docker-dns-srv-test-certs-gateway-run -f ./hack/scripts-dev/Makefile
#   make docker-dns-srv-test-certs-wildcard-run -f ./hack/scripts-dev/Makefile

build-docker-dns-srv-test:
	$(info GO_VERSION: $(GO_VERSION))
	@sed -i.bak 's|REPLACE_ME_GO_VERSION|$(GO_VERSION)|g' ./hack/scripts-dev/docker-dns-srv/Dockerfile
	docker build \
	  --tag gcr.io/etcd-development/etcd-dns-srv-test:go$(GO_VERSION) \
	  --file ./hack/scripts-dev/docker-dns-srv/Dockerfile \
	  ./hack/scripts-dev/docker-dns-srv
	@mv ./hack/scripts-dev/docker-dns-srv/Dockerfile.bak ./hack/scripts-dev/docker-dns-srv/Dockerfile

	docker run \
	  --rm \
	  --dns 127.0.0.1 \
	  gcr.io/etcd-development/etcd-dns-srv-test:go$(GO_VERSION) \
	  /bin/bash -c "/etc/init.d/bind9 start && cat /dev/null >/etc/hosts && dig +noall +answer SRV _etcd-client-ssl._tcp.etcd.local && dig +noall +answer SRV _etcd-server-ssl._tcp.etcd.local && dig +noall +answer m1.etcd.local m2.etcd.local m3.etcd.local"

push-docker-dns-srv-test:
	$(info GO_VERSION: $(GO_VERSION))
	gcloud docker -- push gcr.io/etcd-development/etcd-dns-srv-test:go$(GO_VERSION)

pull-docker-dns-srv-test:
	$(info GO_VERSION: $(GO_VERSION))
	docker pull gcr.io/etcd-development/etcd-dns-srv-test:go$(GO_VERSION)

docker-dns-srv-test-certs-run:
	$(info GO_VERSION: $(GO_VERSION))
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  --tty \
	  --dns 127.0.0.1 \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`/bin,destination=/etcd \
	  --mount type=bind,source=`pwd`/hack/scripts-dev/docker-dns-srv/certs,destination=/certs \
	  gcr.io/etcd-development/etcd-dns-srv-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /certs/run.sh && rm -rf m*.etcd"

docker-dns-srv-test-certs-gateway-run:
	$(info GO_VERSION: $(GO_VERSION))
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  --tty \
	  --dns 127.0.0.1 \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`/bin,destination=/etcd \
	  --mount type=bind,source=`pwd`/hack/scripts-dev/docker-dns-srv/certs-gateway,destination=/certs-gateway \
	  gcr.io/etcd-development/etcd-dns-srv-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /certs-gateway/run.sh && rm -rf m*.etcd"

docker-dns-srv-test-certs-wildcard-run:
	$(info GO_VERSION: $(GO_VERSION))
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  --tty \
	  --dns 127.0.0.1 \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`/bin,destination=/etcd \
	  --mount type=bind,source=`pwd`/hack/scripts-dev/docker-dns-srv/certs-wildcard,destination=/certs-wildcard \
	  gcr.io/etcd-development/etcd-dns-srv-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /certs-wildcard/run.sh && rm -rf m*.etcd"



# Example:
#   make build-etcd-test-proxy -f ./hack/scripts-dev/Makefile

build-etcd-test-proxy:
	go build -v -o ./bin/etcd-test-proxy ./tools/etcd-test-proxy



# Example:
#   make build-docker-functional-tester -f ./hack/scripts-dev/Makefile
#   make push-docker-functional-tester -f ./hack/scripts-dev/Makefile
#   make pull-docker-functional-tester -f ./hack/scripts-dev/Makefile

build-docker-functional-tester:
	$(info GO_VERSION: $(GO_VERSION))
	$(info ETCD_VERSION: $(ETCD_VERSION))
	@sed -i.bak 's|REPLACE_ME_GO_VERSION|$(GO_VERSION)|g' ./Dockerfile-functional-tester
	docker build \
	  --tag gcr.io/etcd-development/etcd-functional-tester:go$(GO_VERSION) \
	  --file ./Dockerfile-functional-tester \
	  .
	@mv ./Dockerfile-functional-tester.bak ./Dockerfile-functional-tester

	docker run \
	  --rm \
	  gcr.io/etcd-development/etcd-functional-tester:go$(GO_VERSION) \
	  /bin/bash -c "/etcd --version && \
	   /etcd-failpoints --version && \
	   ETCDCTL_API=3 /etcdctl version && \
	   /etcd-agent -help || true && \
	   /etcd-tester -help || true && \
	   /etcd-runner --help || true && \
	   /benchmark --help || true && \
	   /etcd-test-proxy -help || true"

push-docker-functional-tester:
	$(info GO_VERSION: $(GO_VERSION))
	$(info ETCD_VERSION: $(ETCD_VERSION))
	gcloud docker -- push gcr.io/etcd-development/etcd-functional-tester:go$(GO_VERSION)

pull-docker-functional-tester:
	$(info GO_VERSION: $(GO_VERSION))
	$(info ETCD_VERSION: $(ETCD_VERSION))
	docker pull gcr.io/etcd-development/etcd-functional-tester:go$(GO_VERSION)

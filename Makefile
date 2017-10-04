# run makefile from repo root

.PHONY: build
build:
	GO_BUILD_FLAGS="-v" ./build
	./bin/etcd --version
	ETCDCTL_API=3 ./bin/etcdctl version

# run all tests
test-all:
	RELEASE_TEST=y INTEGRATION=y PASSES='build unit release integration_e2e functional' ./test 2>&1 | tee test.log

# clean up failed tests, logs, dependencies
clean:
	rm -f ./*.log
	rm -f ./bin/Dockerfile-release
	rm -rf ./bin/*.etcd
	rm -rf ./gopath
	rm -rf ./release
	rm -f ./integration/127.0.0.1:* ./integration/localhost:*
	rm -f ./clientv3/integration/127.0.0.1:* ./clientv3/integration/localhost:*
	rm -f ./clientv3/ordering/127.0.0.1:* ./clientv3/ordering/localhost:*

# keep in-sync with 'Dockerfile-test', 'e2e/docker-dns/Dockerfile',
# 'e2e/docker-dns-srv/Dockerfile'
_GO_VERSION = go1.9.1
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
	docker run \
	  --rm \
	  --volume=`pwd`:/go/src/github.com/coreos/etcd \
	  gcr.io/etcd-development/etcd-test:$(_GO_VERSION) \
	  /bin/bash -c "RELEASE_TEST=y INTEGRATION=y PASSES='build unit release integration_e2e functional' ./test 2>&1 | tee docker-test.log"

docker-test-386:
	docker run \
	  --rm \
	  --volume=`pwd`:/go/src/github.com/coreos/etcd \
	  gcr.io/etcd-development/etcd-test:$(_GO_VERSION) \
	  /bin/bash -c "GOARCH=386 PASSES='build unit integration_e2e' ./test 2>&1 | tee docker-test.log"

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

# build base container image for DNS testing
docker-dns-test-build:
	docker build \
	  --tag gcr.io/etcd-development/etcd-dns-test:$(_GO_VERSION) \
	  --file ./e2e/docker-dns/Dockerfile \
	  ./e2e/docker-dns

	docker run \
	  --rm \
	  --dns 127.0.0.1 \
	  gcr.io/etcd-development/etcd-dns-test:$(_GO_VERSION) \
	  /bin/bash -c "/etc/init.d/bind9 start && cat /dev/null >/etc/hosts && dig etcd.local"

docker-dns-test-push:
	gcloud docker -- push gcr.io/etcd-development/etcd-dns-test:$(_GO_VERSION)

docker-dns-test-pull:
	docker pull gcr.io/etcd-development/etcd-dns-test:$(_GO_VERSION)

# run DNS tests inside container
docker-dns-test-run:
	docker run \
	  --rm \
	  --tty \
	  --dns 127.0.0.1 \
	  --volume=`pwd`/bin:/etcd \
	  --volume=`pwd`/integration/fixtures:/certs \
	  gcr.io/etcd-development/etcd-dns-test:$(_GO_VERSION) \
	  /bin/bash -c "cd /etcd && /run.sh && rm -rf m*.etcd"

# build base container image for DNS/SRV testing
docker-dns-srv-test-build:
	docker build \
	  --tag gcr.io/etcd-development/etcd-dns-srv-test:$(_GO_VERSION) \
	  --file ./e2e/docker-dns-srv/Dockerfile \
	  ./e2e/docker-dns-srv

	docker run \
	  --rm \
	  --dns 127.0.0.1 \
	  gcr.io/etcd-development/etcd-dns-srv-test:$(_GO_VERSION) \
	  /bin/bash -c "/etc/init.d/bind9 start && cat /dev/null >/etc/hosts && dig +noall +answer SRV _etcd-client._tcp.etcd-srv.local && dig +noall +answer SRV _etcd-client-ssl._tcp.etcd-srv.local && dig +noall +answer SRV _etcd-server._tcp.etcd-srv.local && dig +noall +answer SRV _etcd-server-ssl._tcp.etcd-srv.local && dig +noall +answer m1.etcd-srv.local m2.etcd-srv.local m3.etcd-srv.local"

docker-dns-srv-test-push:
	gcloud docker -- push gcr.io/etcd-development/etcd-dns-srv-test:$(_GO_VERSION)

docker-dns-srv-test-pull:
	docker pull gcr.io/etcd-development/etcd-dns-srv-test:$(_GO_VERSION)

# run DNS/SRV tests inside container
docker-dns-srv-test-run:
	docker run \
	  --rm \
	  --tty \
	  --dns 127.0.0.1 \
	  --volume=`pwd`/bin:/etcd \
	  gcr.io/etcd-development/etcd-dns-srv-test:$(_GO_VERSION) \
	  /bin/bash -c "cd /etcd && /run.sh && rm -rf m*.etcd"

# TODO: run DNS/SRV with TLS
# TODO: add DNS integration tests

TMP_DOCKERFILE:=$(shell mktemp)
GO_VERSION ?= 1.17.8
TMP_DIR_MOUNT_FLAG = --tmpfs=/tmp:exec
ifdef HOST_TMP_DIR
	TMP_DIR_MOUNT_FLAG = --mount type=bind,source=$(HOST_TMP_DIR),destination=/tmp
endif

# Example:
#   make build-docker-test
#   make compile-with-docker-test
#   make build-docker-static-ip-test
#
#   gcloud auth configure-docker
#   make push-docker-static-ip-test
#
#   gsutil -m acl ch -u allUsers:R -r gs://artifacts.etcd-development.appspot.com
#   make pull-docker-static-ip-test
#
#   make docker-static-ip-test-certs-run
#   make docker-static-ip-test-certs-metrics-proxy-run

build-docker-static-ip-test:
	$(info GO_VERSION: $(GO_VERSION))
	@sed 's|REPLACE_ME_GO_VERSION|$(GO_VERSION)|g' ./docker-static-ip/Dockerfile > $(TMP_DOCKERFILE)
	docker build \
	  --network=host \
	  --tag gcr.io/etcd-development/etcd-static-ip-test:go$(GO_VERSION) \
	  --file ./docker-static-ip/Dockerfile \
	  $(TMP_DOCKERFILE)

push-docker-static-ip-test:
	$(info GO_VERSION: $(GO_VERSION))
	docker push gcr.io/etcd-development/etcd-static-ip-test:go$(GO_VERSION)

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
	  --mount type=bind,source=`pwd`/tests/docker-static-ip/certs,destination=/certs \
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
	  --mount type=bind,source=`pwd`/tests/docker-static-ip/certs-metrics-proxy,destination=/certs-metrics-proxy \
	  gcr.io/etcd-development/etcd-static-ip-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /certs-metrics-proxy/run.sh && rm -rf m*.etcd"



# Example:
#   make build-docker-test
#   make compile-with-docker-test
#   make build-docker-dns-test
#
#   gcloud auth configure-docker
#   make push-docker-dns-test
#
#   gsutil -m acl ch -u allUsers:R -r gs://artifacts.etcd-development.appspot.com
#   make pull-docker-dns-test
#
#   make docker-dns-test-insecure-run
#   make docker-dns-test-certs-run
#   make docker-dns-test-certs-gateway-run
#   make docker-dns-test-certs-wildcard-run
#   make docker-dns-test-certs-common-name-auth-run
#   make docker-dns-test-certs-common-name-multi-run
#   make docker-dns-test-certs-san-dns-run

build-docker-dns-test:
	$(info GO_VERSION: $(GO_VERSION))
	@sed 's|REPLACE_ME_GO_VERSION|$(GO_VERSION)|g' ./docker-dns/Dockerfile > $(TMP_DOCKERFILE)
	docker build \
	  --network=host \
	  --tag gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION) \
	  --file ./docker-dns/Dockerfile \
	  $(TMP_DOCKERFILE)

	docker run \
	  --rm \
	  --dns 127.0.0.1 \
	  gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION) \
	  /bin/bash -c "/etc/init.d/bind9 start && cat /dev/null >/etc/hosts && dig etcd.local"

push-docker-dns-test:
	$(info GO_VERSION: $(GO_VERSION))
	docker push gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION)

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
	  --mount type=bind,source=`pwd`/tests/docker-dns/insecure,destination=/insecure \
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
	  --mount type=bind,source=`pwd`/tests/docker-dns/certs,destination=/certs \
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
	  --mount type=bind,source=`pwd`/tests/docker-dns/certs-gateway,destination=/certs-gateway \
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
	  --mount type=bind,source=`pwd`/tests/docker-dns/certs-wildcard,destination=/certs-wildcard \
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
	  --mount type=bind,source=`pwd`/tests/docker-dns/certs-common-name-auth,destination=/certs-common-name-auth \
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
	  --mount type=bind,source=`pwd`/tests/docker-dns/certs-common-name-multi,destination=/certs-common-name-multi \
	  gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /certs-common-name-multi/run.sh && rm -rf m*.etcd"

docker-dns-test-certs-san-dns-run:
	$(info GO_VERSION: $(GO_VERSION))
	$(info HOST_TMP_DIR: $(HOST_TMP_DIR))
	$(info TMP_DIR_MOUNT_FLAG: $(TMP_DIR_MOUNT_FLAG))
	docker run \
	  --rm \
	  --tty \
	  --dns 127.0.0.1 \
	  $(TMP_DIR_MOUNT_FLAG) \
	  --mount type=bind,source=`pwd`/bin,destination=/etcd \
	  --mount type=bind,source=`pwd`/tests/docker-dns/certs-san-dns,destination=/certs-san-dns \
	  gcr.io/etcd-development/etcd-dns-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /certs-san-dns/run.sh && rm -rf m*.etcd"


# Example:
#   make build-docker-test
#   make compile-with-docker-test
#   make build-docker-dns-srv-test
#   gcloud auth configure-docker
#   make push-docker-dns-srv-test
#   gsutil -m acl ch -u allUsers:R -r gs://artifacts.etcd-development.appspot.com
#   make pull-docker-dns-srv-test
#   make docker-dns-srv-test-certs-run
#   make docker-dns-srv-test-certs-gateway-run
#   make docker-dns-srv-test-certs-wildcard-run

build-docker-dns-srv-test:
	$(info GO_VERSION: $(GO_VERSION))
	@sed 's|REPLACE_ME_GO_VERSION|$(GO_VERSION)|g' > $(TMP_DOCKERFILE)
	docker build \
	  --network=host \
	  --tag gcr.io/etcd-development/etcd-dns-srv-test:go$(GO_VERSION) \
	  --file ./docker-dns-srv/Dockerfile \
	  $(TMP_DOCKERFILE)

	docker run \
	  --rm \
	  --dns 127.0.0.1 \
	  gcr.io/etcd-development/etcd-dns-srv-test:go$(GO_VERSION) \
	  /bin/bash -c "/etc/init.d/bind9 start && cat /dev/null >/etc/hosts && dig +noall +answer SRV _etcd-client-ssl._tcp.etcd.local && dig +noall +answer SRV _etcd-server-ssl._tcp.etcd.local && dig +noall +answer m1.etcd.local m2.etcd.local m3.etcd.local"

push-docker-dns-srv-test:
	$(info GO_VERSION: $(GO_VERSION))
	docker push gcr.io/etcd-development/etcd-dns-srv-test:go$(GO_VERSION)

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
	  --mount type=bind,source=`pwd`/tests/docker-dns-srv/certs,destination=/certs \
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
	  --mount type=bind,source=`pwd`/tests/docker-dns-srv/certs-gateway,destination=/certs-gateway \
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
	  --mount type=bind,source=`pwd`/tests/docker-dns-srv/certs-wildcard,destination=/certs-wildcard \
	  gcr.io/etcd-development/etcd-dns-srv-test:go$(GO_VERSION) \
	  /bin/bash -c "cd /etcd && /certs-wildcard/run.sh && rm -rf m*.etcd"

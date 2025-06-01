REPOSITORY_ROOT := $(shell git rev-parse --show-toplevel)
USER_ID := $(shell id -u)
GROUP_ID := $(shell id -g)
ARCH ?= $(shell go env GOARCH)
REF = main
IMAGE_TAG = latest

.PHONY: antithesis-build-client-docker-image
antithesis-build-client-docker-image:
	docker build --build-arg GO_VERSION=$(shell cat $(REPOSITORY_ROOT)/.go-version) -f $(REPOSITORY_ROOT)/tests/antithesis/test-template/Dockerfile $(REPOSITORY_ROOT) -t etcd-client:latest

.PHONY: antithesis-build-etcd-image
antithesis-build-etcd-image:
	docker build --build-arg GO_VERSION=$(shell cat $(REPOSITORY_ROOT)/.go-version) --build-arg REF=$(REF) $(REPOSITORY_ROOT)/tests/antithesis/server/ -t etcd-server:latest

.PHONY: antithesis-build-etcd-image-release-3.4
antithesis-build-etcd-image-release-3.4: REF=release-3.4
antithesis-build-etcd-image-release-3.4: antithesis-build-etcd-image

.PHONY: antithesis-build-etcd-image-release-3.5
antithesis-build-etcd-image-release-3.5: REF=release-3.5
antithesis-build-etcd-image-release-3.5: antithesis-build-etcd-image

.PHONY: antithesis-build-etcd-image-release-3.6
antithesis-build-etcd-image-release-3.6: REF=release-3.6
antithesis-build-etcd-image-release-3.6: antithesis-build-etcd-image

.PHONY: antithesis-build-etcd-image-main
antithesis-build-etcd-image-main: REF=main
antithesis-build-etcd-image-main: antithesis-build-etcd-image

.PHONY: antithesis-build-config-image
antithesis-build-config-image:
	docker build -f ./Dockerfile.config . -t etcd-config:latest --build-arg IMAGE_TAG=$(IMAGE_TAG)

.PHONY: antithesis-docker-compose-up
antithesis-docker-compose-up:
	export USER_ID=$(USER_ID) && export GROUP_ID=$(GROUP_ID) && docker-compose up

.PHONY: antithesis-run-container-traffic
antithesis-run-container-traffic:
	export USER_ID=$(USER_ID) && export GROUP_ID=$(GROUP_ID) && docker-compose exec client /opt/antithesis/test/v1/robustness/singleton_driver_traffic

.PHONY: antithesis-run-container-validation
antithesis-run-container-validation:
	export USER_ID=$(USER_ID) && export GROUP_ID=$(GROUP_ID) && docker-compose exec client /opt/antithesis/test/v1/robustness/finally_validation

.PHONY: antithesis-run-local-traffic
antithesis-run-local-traffic:
	go run --race ./test-template/robustness/traffic/main.go --local

.PHONY: antithesis-run-local-validation
antithesis-run-local-validation:
	go run --race ./test-template/robustness/finally/main.go --local

.PHONY: antithesis-clean
antithesis-clean:
	export USER_ID=$(USER_ID) && export GROUP_ID=$(GROUP_ID) && docker-compose down
	rm -rf /tmp/etcddata0 /tmp/etcddata1 /tmp/etcddata2 /tmp/etcdreport

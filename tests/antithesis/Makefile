ANTITHESIS_ROOT :=$(dir $(realpath $(lastword $(MAKEFILE_LIST))))
REPOSITORY_ROOT := $(shell git rev-parse --show-toplevel)
USER_ID := $(shell id -u)
GROUP_ID := $(shell id -g)
ARCH ?= $(shell go env GOARCH)
REF = main
IMAGE_TAG = latest

CFG_NODE_COUNT ?= 3

.PHONY: antithesis-build-client-docker-image
antithesis-build-client-docker-image: validate-node-count
	# This is a release image using go build without our tooling. Using the
	# version defined in .go-version is the correct image tag.
	docker build \
		--build-arg GO_IMAGE_TAG=$(shell cat $(REPOSITORY_ROOT)/.go-version) \
		--build-arg CFG_NODE_COUNT=$(CFG_NODE_COUNT) \
		--file $(ANTITHESIS_ROOT)/test-template/Dockerfile \
		--tag etcd-client:latest \
		$(REPOSITORY_ROOT)

.PHONY: antithesis-build-etcd-image
antithesis-build-etcd-image:
	# Use the Workspace Go version from the go directive. This is the base image
	# later in the Dockerfile, we'll set the correct toolchain.
	docker build \
		--build-arg GO_IMAGE_TAG=$(shell go work edit -json | jq .Go) \
		--build-arg REF=$(REF) \
		--tag etcd-server:latest \
		$(ANTITHESIS_ROOT)/server/

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
antithesis-build-config-image: validate-node-count
	docker build -f config/Dockerfile config -t etcd-config:latest \
		--build-arg IMAGE_TAG=$(IMAGE_TAG) \
		--build-arg NODE_COUNT=$(CFG_NODE_COUNT)

.PHONY: antithesis-docker-compose-up
antithesis-docker-compose-up: validate-node-count
	export USER_ID=$(USER_ID) && export GROUP_ID=$(GROUP_ID) && \
		docker compose -f config/docker-compose-$(CFG_NODE_COUNT)-node.yml up

.PHONY: antithesis-run-container-traffic
antithesis-run-container-traffic: validate-node-count
	export USER_ID=$(USER_ID) && export GROUP_ID=$(GROUP_ID) && \
		docker compose -f config/docker-compose-$(CFG_NODE_COUNT)-node.yml exec client /opt/antithesis/test/v1/robustness/singleton_driver_traffic

.PHONY: antithesis-run-container-validation
antithesis-run-container-validation: validate-node-count
	export USER_ID=$(USER_ID) && export GROUP_ID=$(GROUP_ID) && \
		docker compose -f config/docker-compose-$(CFG_NODE_COUNT)-node.yml exec client /opt/antithesis/test/v1/robustness/finally_validation

.PHONY: antithesis-run-local-traffic
antithesis-run-local-traffic:
	go run -ldflags "-X main.NodeCount=$(CFG_NODE_COUNT)" --race ./test-template/robustness/traffic/main.go --local

.PHONY: antithesis-run-local-validation
antithesis-run-local-validation:
	go run -ldflags "-X main.NodeCount=$(CFG_NODE_COUNT)" --race ./test-template/robustness/finally/main.go --local

.PHONY: antithesis-clean
antithesis-clean: validate-node-count
	export USER_ID=$(USER_ID) && export GROUP_ID=$(GROUP_ID) && \
		docker compose -f config/docker-compose-$(CFG_NODE_COUNT)-node.yml down --remove-orphans
	rm -rf /tmp/etcddata0 /tmp/etcddata1 /tmp/etcddata2 /tmp/etcdreport

.PHONY: validate-node-count
validate-node-count:
	@if [ "$(CFG_NODE_COUNT)" != "1" ] && [ "$(CFG_NODE_COUNT)" != "3" ]; then \
		echo "CFG_NODE_COUNT must be either 1 or 3 (got $(CFG_NODE_COUNT))"; \
		exit 1; \
	fi

REQUIRED_K8S_TOOLS := kubectl kind

.PHONY: check-k8s-tools
check-k8s-tools:
	@for tool in $(REQUIRED_K8S_TOOLS); do \
		if ! command -v $$tool >/dev/null 2>&1; then \
			echo "Error: '$$tool' is missing.  Please install it before continuing." >&2; \
			exit 1; \
		fi \
	done

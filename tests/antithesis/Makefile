REPOSITORY_ROOT := $(shell git rev-parse --show-toplevel)
USER_ID := $(shell id -u)
GROUP_ID := $(shell id -g)
ARCH ?= $(shell go env GOARCH)
REF = HEAD
DOCKERFILE = ./Dockerfile
REMOTE = origin

.PHONY: antithesis-build-client-docker-image
antithesis-build-client-docker-image:
	docker build --build-arg GO_VERSION=$(shell cat $(REPOSITORY_ROOT)/.go-version) -f $(REPOSITORY_ROOT)/tests/antithesis/test-template/Dockerfile $(REPOSITORY_ROOT) -t etcd-client:latest

.PHONY: antithesis-build-etcd-create-worktree
antithesis-build-etcd-create-worktree:
	git fetch $(REMOTE) $(REF)
	git worktree remove -f /tmp/etcd$(REF) || true
	git worktree add /tmp/etcd$(REF) $(REMOTE)/$(REF)
	$(eval BUILDDIR=/tmp/etcd$(REF))

.PHONY: antithesis-build-etcd
antithesis-build-etcd:
	cd $(BUILDDIR) && make build
	cd $(BUILDDIR) && cp $(DOCKERFILE) ./bin/Dockerfile
	cd $(BUILDDIR) && docker build \
		--tag etcd-server:latest \
		./bin
	rm $(BUILDDIR)/bin/Dockerfile

.PHONY: antithesis-build-etcd-remove-worktree
antithesis-build-etcd-remove-worktree:
	git worktree remove -f $(BUILDDIR)

.PHONY: antithesis-build-etcd-image
antithesis-build-etcd-image: antithesis-build-etcd-create-worktree antithesis-build-etcd  antithesis-build-etcd-remove-worktree

.PHONY: antithesis-build-etcd-image-release-3.4
antithesis-build-etcd-image-release-3.4: set-version-3.4 antithesis-build-etcd-image

.PHONY: antithesis-build-etcd-image-release-3.5
antithesis-build-etcd-image-release-3.5: set-version-3.5 antithesis-build-etcd-image

.PHONY: antithesis-build-etcd-image-release-3.6
antithesis-build-etcd-image-release-3.6: set-version-3.6 antithesis-build-etcd-image

.PHONY: antithesis-build-etcd-image-main
antithesis-build-etcd-image-main: set-version-main antithesis-build-etcd-image

.PHONY: antithesis-build-etcd-image-local
antithesis-build-etcd-image-local: set-local antithesis-build-etcd

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

.PHONY: set-version-3.4
set-version-3.4:
	$(eval REF=release-3.4)
	$(eval DOCKERFILE=./Dockerfile-release)

.PHONY: set-version-3.5
set-version-3.5:
	$(eval REF=release-3.5)
	$(eval DOCKERFILE=./Dockerfile-release.amd64)

.PHONY: set-version-3.6
set-version-3.6:
	$(eval REF=release-3.6)
	$(eval DOCKERFILE=./Dockerfile)

.PHONY: set-version-main
set-version-main:
	$(eval REF=main)
	$(eval DOCKERFILE=./Dockerfile)

.PHONY: set-local
set-local:
	$(eval BUILDDIR=$(REPOSITORY_ROOT))

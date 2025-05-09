REPOSITORY_ROOT := $(shell git rev-parse --show-toplevel)
USER_ID := $(shell id -u)
GROUP_ID := $(shell id -g)

.PHONY: antithesis-build-client-docker-image
antithesis-build-client-docker-image:
	docker build --build-arg GO_VERSION=$(shell cat $(REPOSITORY_ROOT)/.go-version) -f $(REPOSITORY_ROOT)/tests/antithesis/test-template/Dockerfile $(REPOSITORY_ROOT) -t etcd-client:latest

.PHONY: antithesis-docker-compose-up
antithesis-docker-compose-up:
	export USER_ID=$(USER_ID) && export GROUP_ID=$(GROUP_ID) && docker-compose up

.PHONY: antithesis-run-container-tests
antithesis-run-container-tests:
	export USER_ID=$(USER_ID) && export GROUP_ID=$(GROUP_ID) && docker-compose exec client /opt/antithesis/test/v1/robustness/singleton_driver_main

.PHONY: antithesis-run-local-tests
antithesis-run-local-tests:
	go run ./test-template/robustness/main.go --local

.PHONY: antithesis-clean
antithesis-clean:
	export USER_ID=$(USER_ID) && export GROUP_ID=$(GROUP_ID) && docker-compose down
	rm -rf /tmp/etcddata0 /tmp/etcddata1 /tmp/etcddata2 /tmp/etcdreport

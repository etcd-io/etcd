REPOSITORY_ROOT := $(shell git rev-parse --show-toplevel)

.PHONY: antithesis-build-client-docker-image
antithesis-build-client-docker-image:
	docker build --build-arg GO_VERSION=$(shell cat $(REPOSITORY_ROOT)/.go-version) -f $(REPOSITORY_ROOT)/tests/antithesis/test-template/Dockerfile $(REPOSITORY_ROOT)

PACKAGE := $(shell go list)
GOOS := $(shell go env GOOS)
GOARCH = $(shell go env GOARCH)
OBJ_DIR := $(GOPATH)/pkg/$(GOOS)_$(GOARCH)/$(PACKAGE)

# Dependencies Management
.PHONY: vendor-prepare
vendor-prepare:
	@echo "Installing dep"
	@go get -u -v github.com/golang/dep/cmd/dep

Gopkg.lock: Gopkg.toml
	@dep ensure -update $(DEP_OPTS)

.PHONY: vendor-update
vendor-update:
	@dep ensure -update $(DEP_OPTS)

vendor: Gopkg.lock
	@dep ensure $(DEP_OPTS)

vendor-optimize: vendor
	@dep prune

.PHONY: clean-vendor
clean-vendor:
	@rm -rf vendor

# Linter
.PHONY: lint-prepare
lint-prepare:
	@echo "Installing golangci-lint"
	@go get -u github.com/golangci/golangci-lint/cmd/golangci-lint

.PHONY: lint
lint: vendor
	@golangci-lint run \
		--exclude-use-default=false \
		--enable=golint \
		--enable=gocyclo \
		--enable=goconst \
		--enable=unconvert \
		--exclude='^Error return value of `.*\.Log` is not checked$$' \
		--exclude='^G104: Errors unhandled\.$$' \
		--exclude='^G304: Potential file inclusion via variable$$' \
		./...

# Testing
.PHONY: test
test: vendor
	@go test $(TEST_OPTS)

.PHONY: bench
bench: vendor
	@go test -run=none -bench=. -benchmem

# Build and Installation
.PHONY: install
install: vendor
	@go install ./...

.PHONY: uninstall
uninstall:
	@echo "Removing binaries and libraries"
	@go clean -i ./...
	@if [ -d $(OBJ_DIR) ]; then \
		rm -rf $(OBJ_DIR); \
	fi

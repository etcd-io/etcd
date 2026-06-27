# --- Required ----------------------------------------------------------------
export PATH   := $(PWD)/bin:$(PATH)                    # ./bin to $PATH
export SHELL  := bash                                  # Default Shell

define install_go_bin
	@ which $(1) 2>&1 1>/dev/null || GOBIN=$(PWD)/bin go install $(2)
endef

.DEFAULT_GOAL := help

# Generate Artifacts ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
generate: ## Generate Assets
	$(MAKE) generate-tests
	$(MAKE) generate-mirror-table

generate-tests: ## Generates Assets at testdata
	go run ./cmd/internal/tests/ "$(PWD)/testdata"

generate-mirror-table: ## Generate Asset MIRROR_FUNCS.md
	go run ./cmd/internal/mirror-table/ > "$(PWD)/MIRROR_FUNCS.md"


# Build Artifacts ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
build: ## Build binary
	@ go build -trimpath -ldflags="-w -s" -o bin/mirror ./cmd/mirror/

build-race: ## Build binary with race flag
	@ go build -race -trimpath -ldflags="-w -s" -o bin/mirror ./cmd/mirror/

install: ## Installs binary
	@ go install -trimpath -v -ldflags="-w -s" ./cmd/mirror

# Run Tests ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
tests: ## Run Tests (Summary)
	@ go test -v -count=1 -race \
		-failfast \
		-parallel=2 \
		-timeout=1m \
		-covermode=atomic \
	    -coverprofile=coverage.cov ./...

tests-summary: ## Run Tests, but shows summary
tests-summary: bin/tparse
	@ go test -v -count=1 -race \
		-failfast \
		-parallel=2 \
		-timeout=1m \
		-covermode=atomic \
		-coverprofile=coverage.cov --json ./... | tparse -all

# Linter ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

lints: ## Run golangci-lint
lints: bin/golangci-lint
lints:
	golangci-lint run --no-config ./... --exclude-dirs "^(cmd|testdata)"


cover: ## Run Coverage
	@ go tool cover -html=coverage.cov

# Other ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

test-release: bin/goreleaser
	goreleaser release --help
	goreleaser release --skip=publish --skip=validate --clean

# Install  ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

bin/tparse: ## Installs tparse@v0.13.2 (if not exists)
bin/tparse: INSTALL_URL=github.com/mfridman/tparse@v0.13.2
bin/tparse:
	$(call install_go_bin, tparse, $(INSTALL_URL))

bin/golangci-lint: ## Installs golangci-lint@v1.62.0 (if not exists)
bin/golangci-lint: INSTALL_URL=github.com/golangci/golangci-lint@v1.62.0
bin/golangci-lint:
	$(call install_go_bin, golangci-lint, $(INSTALL_URL))

bin/goreleaser: ## Installs goreleaser@v1.24.0 (if not exists)
bin/goreleaser: INSTALL_URL=github.com/goreleaser/goreleaser@v1.24.0
bin/goreleaser:
	$(call install_go_bin, goreleaser, $(INSTALL_URL))

# Help ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

help: dep-gawk
	@ echo "=============================================================================="
	@ echo " Makefile: github.com/butuzov/mirror                                   "
	@ echo "=============================================================================="
	@ cat $(MAKEFILE_LIST) | \
		grep -E '^# ~~~ .*? [~]+$$|^[a-zA-Z0-9_-]+:.*?## .*$$' | \
		gawk '{if ( $$1=="#" ) { \
			match($$0, /^# ~~~ (.+?) [~]+$$/, a);\
			{print "\n", a[1], ""}\
		} else { \
			match($$0, /^([a-zA-Z/_-]+):.*?## (.*)$$/, a); \
			{printf "  - \033[32m%-20s\033[0m %s\n",   a[1], a[2]} \
 		}}'
	@ echo ""


# Helper Methods ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
dep-gawk:
	@ if [ -z "$(shell command -v gawk)" ]; then  \
		if [ -x /usr/local/bin/brew ]; then $(MAKE) _brew_gawk_install; exit 0; fi; \
		if [ -x /usr/bin/apt-get ]; then $(MAKE) _ubuntu_gawk_install; exit 0; fi; \
		if [ -x /usr/bin/yum ]; then  $(MAKE) _centos_gawk_install; exit 0; fi; \
		if [ -x /sbin/apk ]; then  $(MAKE) _alpine_gawk_install; exit 0; fi; \
		echo  "GNU Awk Required.";\
		exit 1; \
	fi

_brew_gawk_install:
	@ echo "Installing gawk using brew... "
	@ brew install gawk --quiet
	@ echo "done"

_ubuntu_gawk_install:
	@ echo "Installing gawk using apt-get... "
	@ apt-get -q install gawk -y
	@ echo "done"

_alpine_gawk_install:
	@ echo "Installing gawk using yum... "
	@ apk add --update --no-cache gawk
	@ echo "done"

_centos_gawk_install:
	@ echo "Installing gawk using yum... "
	@ yum install -q -y gawk;
	@ echo "done"

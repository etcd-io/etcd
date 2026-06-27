# Git remote for pushing tags
REMOTE ?= origin

# Version for release tagging (required for tag/release targets)
RELEASE_VERSION ?=

# Convenience
GO        ?= go
GOLANGCI  ?= golangci-lint
GORELEASER?= goreleaser

.PHONY: help \
        test race bench fmt tidy lint check \
        ensure-clean ensure-release-version tag tag-delete \
        release release-dry

help:
	@echo "Targets:"
	@echo "  fmt           - gofmt + go fmt"
	@echo "  tidy          - go mod tidy"
	@echo "  test          - go test ./..."
	@echo "  race          - go test -race ./..."
	@echo "  bench         - go test -bench=. ./..."
	@echo "  lint          - golangci-lint run ./... (if installed)"
	@echo "  check         - fmt + tidy + test + race"
	@echo ""
	@echo "Release targets:"
	@echo "  tag           - Create annotated tag RELEASE_VERSION and push"
	@echo "  tag-delete    - Delete tag RELEASE_VERSION locally + remote"
	@echo "  release       - tag + goreleaser release --clean (if you use goreleaser)"
	@echo "  release-dry   - tag + goreleaser release --clean --skip=publish"
	@echo ""
	@echo "Usage:"
	@echo "  make check"
	@echo "  make tag RELEASE_VERSION=v0.1.2"
	@echo "  make release RELEASE_VERSION=v0.1.2"

fmt:
	@echo "Formatting..."
	gofmt -w -s .
	$(GO) fmt ./...

tidy:
	@echo "Tidying..."
	$(GO) mod tidy

test:
	@echo "Testing..."
	$(GO) test ./... -count=1

race:
	@echo "Race testing..."
	$(GO) test ./... -race -count=1

bench:
	@echo "Bench..."
	$(GO) test ./... -bench=. -run=^$$

lint:
	@echo "Linting..."
	@command -v $(GOLANGCI) >/dev/null 2>&1 || { echo "golangci-lint not found"; exit 1; }
	$(GOLANGCI) run ./...

check: fmt tidy test race

# --------------------------
# Release helpers
# --------------------------

ensure-clean:
	@echo "Checking git working tree..."
	@git diff --quiet || (echo "Error: tracked changes exist. Commit/stash them."; exit 1)
	@test -z "$$(git status --porcelain)" || (echo "Error: uncommitted/untracked files:"; git status --porcelain; exit 1)
	@echo "OK: working tree clean"

ensure-release-version:
	@test -n "$(RELEASE_VERSION)" || (echo "Error: set RELEASE_VERSION, e.g. make tag RELEASE_VERSION=v0.1.2"; exit 1)

tag: ensure-clean ensure-release-version
	@if git rev-parse "$(RELEASE_VERSION)" >/dev/null 2>&1; then \
		echo "Error: tag $(RELEASE_VERSION) already exists. Bump version."; \
		exit 1; \
	fi
	@echo "Tagging $(RELEASE_VERSION) at HEAD $$(git rev-parse --short HEAD)"
	@git tag -a $(RELEASE_VERSION) -m "$(RELEASE_VERSION)"
	@git push $(REMOTE) $(RELEASE_VERSION)

tag-delete: ensure-release-version
	@echo "Deleting tag $(RELEASE_VERSION) locally + remote..."
	@git tag -d $(RELEASE_VERSION) 2>/dev/null || true
	@git push $(REMOTE) :refs/tags/$(RELEASE_VERSION) || true

release: tag
	@command -v $(GORELEASER) >/dev/null 2>&1 || { echo "goreleaser not found"; exit 1; }
	$(GORELEASER) release --clean

release-dry: tag
	@command -v $(GORELEASER) >/dev/null 2>&1 || { echo "goreleaser not found"; exit 1; }
	$(GORELEASER) release --clean --skip=publish

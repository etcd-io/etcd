current_dir = $(shell pwd)

.PHONY: goimports
goimports:
	find . -name '*.go' -exec goimports -w -local github.com/ryancurrah/gomodguard {} +

.PHONY: lint
lint:
	golangci-lint run ./...
	cd cmd/gomodguard && golangci-lint run ./...

.PHONY: tidy
tidy:
	go mod tidy
	cd cmd/gomodguard && go mod tidy

.PHONY: build
build:
	cd cmd/gomodguard && go build -o "$$(go env GOPATH)/bin/gomodguard" main.go

.PHONY: run
run: build
	./gomodguard

.PHONY: test
test:
	go test -v -coverprofile coverage.out
	cd cmd/gomodguard && go test -v -coverprofile coverage.out ./...
	cat cmd/gomodguard/coverage.out | tail -n +2 >> coverage.out

.PHONY: cover
cover:
	gocover-cobertura < coverage.out > coverage.xml

.PHONY: dockerrun
dockerrun: dockerbuild
	docker run -v "${current_dir}/.gomodguard.yaml:/.gomodguard.yaml" ryancurrah/gomodguard:latest

.PHONY: snapshot
snapshot:
	cd cmd/gomodguard && goreleaser --clean --snapshot

.PHONY: release
release:
	cd cmd/gomodguard && goreleaser --clean

.PHONY: clean
clean:
	rm -rf dist/
	rm -f gomodguard coverage.xml coverage.out
	rm -f cmd/gomodguard/coverage.out

.PHONY: tag
tag:
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "error: working tree not clean"; exit 1; \
	fi; \
	current=$$(git tag --sort=-v:refname --list 'v*' | head -n1 || echo "none"); \
	read -p "Current version: $$current. Enter new version: " version; \
	if [ -z "$$version" ]; then echo "error: version required"; exit 1; fi; \
	bump_branch="bump-library-to-$$version"; \
	git tag "$$version" && \
	git push origin "$$version" && \
	git checkout -b "$$bump_branch" && \
	(cd cmd/gomodguard && GOWORK=off go get "github.com/ryancurrah/gomodguard/v2@$$version" && GOWORK=off go mod tidy) && \
	git add cmd/gomodguard/go.mod cmd/gomodguard/go.sum && \
	git commit -m "chore: bump library to $$version" && \
	git tag "cmd/gomodguard/$$version" && \
	git push -u origin "$$bump_branch" "cmd/gomodguard/$$version" && \
	gh pr create --title "chore: bump library to $$version" --body "Required by cmd/gomodguard/$$version release." && \
	echo "waiting for PR to merge..." && \
	while :; do \
		state=$$(gh pr view --json state -q .state); \
		if [ "$$state" = "MERGED" ]; then break; fi; \
		if [ "$$state" = "CLOSED" ]; then echo "error: PR closed without merge"; exit 1; fi; \
		sleep 30; \
	done && \
	git checkout main && git pull --ff-only origin main && \
	git branch -D "$$bump_branch"

.PHONY: install-mac-tools
install-tools-mac:
	brew install goreleaser/tap/goreleaser

.PHONY: install-go-tools
install-go-tools:
	go install -v github.com/t-yuki/gocover-cobertura@latest

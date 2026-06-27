## Help display.
## Pulls comments from beside commands and prints a nicely formatted
## display with the commands and their usage information.

.DEFAULT_GOAL := help

help: ## Prints this help
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: lint
lint: ## Lint the application
	golangci-lint run --max-same-issues=0 --timeout=1m ./...

.PHONY: test
test: ## Run unit tests
	go test -race -shuffle=on ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

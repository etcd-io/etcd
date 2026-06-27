# default task since it's first
.PHONY: all
all: build test

BINARY = importas
$(BINARY): *.go go.mod go.sum
	go build -o $(BINARY)

.PHONY: build
build: $(BINARY) ## Build binary

.PHONY: test
test: build ## Unit test
	go test -v ./...

install: ## Install binary
	go install

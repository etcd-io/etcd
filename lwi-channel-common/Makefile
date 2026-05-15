
GO_DIRS=pkg

.PHONY: proto
proto:
	mkdir -p pkg/mp-dp-proto
	protoc -I proto \
		--go_out=pkg/mp-dp-proto \
		--go_opt=paths=source_relative \
		proto/*.proto

#
# applies gofmt formatting
#
.PHONY: fmt
fmt:
	gofmt -l -w -s $(GO_DIRS)

#
# lists files that need formatting
#
.PHONY: fmtcheck
fmtcheck:
	@unformatted=$$(gofmt -l $(GO_DIRS)); \
	test -z "$$unformatted" || { echo "unformatted files:"; echo "$$unformatted"; exit 1; }

build: proto fmt
	go mod tidy
	go build ./...

.PHONY: proto build
.PHONY:

test:
	go test -v -race ./...

linter:
	golangci-lint -v run ./...

generate:
	go run ./cmd/initialismer/*.go -target="mapping" > ./initialism.go
	go run ./cmd/initialismer/*.go -target="test" > ./testdata/src/initialism/initialism.go
	gofmt -w ./initialism.go ./testdata/src/initialism/initialism.go

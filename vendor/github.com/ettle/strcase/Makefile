.PHONY: benchmark docs lint test

docs:
	which godoc2ghmd || go get github.com/DevotedHealth/godoc2ghmd
	godoc2ghmd -template .readme.tmpl github.com/ettle/strcase > README.md
	go mod tidy

test:
	go test -cover ./...

lint:
	which golangci-lint || go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.1
	golangci-lint run
	golangci-lint run benchmark/*.go
	go mod tidy

benchmark:
	cd benchmark && go test -bench=. -test.benchmem
	go mod tidy

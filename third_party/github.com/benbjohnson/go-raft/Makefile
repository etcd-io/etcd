all: test

coverage:
	gocov test github.com/benbjohnson/go-raft | gocov-html > coverage.html
	open coverage.html

dependencies:
	go get -d .

test:
	go test -v ./...

.PHONY: coverage dependencies test

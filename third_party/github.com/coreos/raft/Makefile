COVERPROFILE=cover.out

default: test

cover:
	go test -coverprofile=$(COVERPROFILE) .
	go tool cover -html=$(COVERPROFILE)
	rm $(COVERPROFILE)

dependencies:
	go get -d .

test:
	go test -i ./...
	go test -v ./...

.PHONY: coverage dependencies test

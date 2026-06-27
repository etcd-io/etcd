CONTAINER=golangci/misspell

default: generate lint test build

install:  ## install misspell into GOPATH/bin
	go install ./cmd/misspell

build:  ## build misspell
	go build ./cmd/misspell

test:  ## run all tests
	CGO_ENABLED=1 go test -v -race .

lint:  ## run linter
	golangci-lint run

generate:
	go run ./internal/gen/

# the grep in line 2 is to remove misspellings in the spelling dictionary
# that trigger false positives!!
falsepositives: /scowl-wl
	cat /scowl-wl/words-US-60.txt | \
		grep -i -v -E "payed|Tyre|Euclidian|nonoccurence|dependancy|reenforced|accidently|surprize|dependance|idealogy|binominal|causalities|conquerer|withing|casette|analyse|analogue|dialogue|paralyse|catalogue|archaeolog|clarinettist|catalyses|cancell|chisell|ageing|cataloguing" | \
		misspell -debug -error
	cat /scowl-wl/words-GB-ise-60.txt | \
		grep -v -E "payed|nonoccurence|withing" | \
		misspell -locale=UK -debug -error
#	cat /scowl-wl/words-GB-ize-60.txt | \
#		grep -v -E "withing" | \
#		misspell -debug -error
#	cat /scowl-wl/words-CA-60.txt | \
#		grep -v -E "withing" | \
#		misspell -debug -error

bench:  ## run benchmarks
	go test -bench '.*'

clean:  ## clean up time
	rm -rf dist/ bin/
	go clean ./...
	git gc --aggressive

ci: docker-build ## run test like travis-ci does, requires docker
	docker run --rm \
		-v $(PWD):/go/src/github.com/golangci/misspell \
		-w /go/src/github.com/golangci/misspell \
		${CONTAINER} \
		make install falsepositives

docker-build:  ## build a docker test image
	docker build -t ${CONTAINER} .

docker-console:  ## log into the test image
	docker run --rm -it \
		-v $(PWD):/go/src/github.com/golangci/misspell \
		-w /go/src/github.com/golangci/misspell \
		${CONTAINER} sh

.PHONY: help ci console docker-build bench

# https://www.client9.com/self-documenting-makefiles/
help:
	@awk -F ':|##' '/^[^\t].+?:.*?##/ {\
	printf "\033[36m%-30s\033[0m %s\n", $$1, $$NF \
	}' $(MAKEFILE_LIST)
.DEFAULT_GOAL=default
.PHONY=help


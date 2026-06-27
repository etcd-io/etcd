SRCS=$(filter-out %_test.go, $(wildcard *.go */*.go))
include integration/makefile.mk

.PHONY: all
all: gofail

.PHONY: clean
clean:
	rm -f gofail

verify: verify-gofmt

.PHONY: verify-gofmt
verify-gofmt:
	@echo "Verifying gofmt, failures can be fixed with 'make fix'"
	@!(gofmt -l -s -d . | grep '[a-z]')

.PHONY: test
test:
	go test -v --race -cpu=1,2,4 ./code/ ./runtime/

fix: fix-gofmt

.PHONY: fix-gofmt
fix-gofmt:
	gofmt -w .

gofail: 
	GO_BUILD_FLAGS="-v" ./build.sh
	./gofail --version

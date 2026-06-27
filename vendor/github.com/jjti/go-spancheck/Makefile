.PHONY: fmt
fmt:
	golangci-lint run --fix --config ./.golangci.yml

.PHONY: test
test: testvendor
	go test -v ./...

# note: I'm copying https://github.com/ghostiam/protogetter/blob/main/testdata/Makefile
#
# x/tools/go/analysis/analysistest does not support go modules. To work around this issue
# we need to vendor any external modules to `./src`.
#
# Follow https://github.com/golang/go/issues/37054 for more details.
.PHONY: testvendor
testvendor:
	rm -rf testdata/base/src
	cd testdata/base && GOWORK=off go mod vendor
	cp -r testdata/base/vendor testdata/base/src
	cp -r testdata/base/vendor testdata/disableerrorchecks/src
	cp -r testdata/base/vendor testdata/enableall/src
	rm -rf testdata/base/vendor

.PHONY: install
install:
	go install ./cmd/spancheck
	@echo "Installed in $(shell which spancheck)"
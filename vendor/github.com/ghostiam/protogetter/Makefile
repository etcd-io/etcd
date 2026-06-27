.PHONY: test
test:
	$(MAKE) -C testdata vendor
	go test -v ./...

.PHONY: install
install:
	go install ./cmd/protogetter
	@echo "Installed in $(shell which protogetter)"

bin/license-bill-of-materials:
	GOBIN=$(PWD)/bin go install -v .

.PHONY: test
test:
	go test -v -i ./...
	go test -v ./...

.PHONY: generate
generate: bin/license-bill-of-materials
	go generate ./...
	./bin/license-bill-of-materials . > bill-of-materials.json

.PHONY: verify-generate
verify-generate: generate
	./scripts/git-diff

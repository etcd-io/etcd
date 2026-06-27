.POSIX:
.SUFFIXES:

all: test lint

test:
	go test -race -shuffle=on -cover ./...

test/cover:
	go test -race -shuffle=on -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint:
	golangci-lint run

tidy:
	go mod tidy

generate:
	go generate ./...

# run `make pre-commit` once to install the hook.
pre-commit: .git/hooks/pre-commit test lint tidy generate
	git diff --exit-code

.git/hooks/pre-commit:
	echo "make pre-commit" > .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit

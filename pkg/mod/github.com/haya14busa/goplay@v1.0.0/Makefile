# Prefer go commands for basic tasks like `build`, `test`, etc...

.PHONY: deps lint release

deps:
	go get -d -v -t ./...
	go get github.com/mattn/goveralls
	go get golang.org/x/tools/cmd/cover
	go get github.com/golang/lint/golint
	go get github.com/kisielk/errcheck
	go get github.com/client9/misspell/cmd/misspell

lint: deps
	! gofmt -d -s . | grep '^' # exit 1 if any output given
	! golint ./... | grep '^'
	go vet ./...
	errcheck -asserts -ignoretests -ignore 'Close'
	misspell -error **/*.go **/*.md

# https://github.com/golang/go/wiki/HostedContinuousIntegration
# https://loads.pickle.me.uk/2015/08/22/easy-peasy-github-releases-for-go-projects-using-travis/
package = github.com/haya14busa/goplay/cmd/goplay

release: deps
	mkdir -p release
	GOOS=linux   GOARCH=amd64 go build -o release/goplay-linux-amd64       $(package)
	GOOS=linux   GOARCH=386   go build -o release/goplay-linux-386         $(package)
	GOOS=linux   GOARCH=arm   go build -o release/goplay-linux-arm         $(package)
	GOOS=windows GOARCH=amd64 go build -o release/goplay-windows-amd64.exe $(package)
	GOOS=windows GOARCH=386   go build -o release/goplay-windows-386.exe   $(package)
	GOOS=darwin  GOARCH=amd64 go build -o release/goplay-darwin-amd64      $(package)
	GOOS=darwin  GOARCH=386   go build -o release/goplay-darwin-386        $(package)

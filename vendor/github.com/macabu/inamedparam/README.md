# inamedparam

A linter that reports interfaces with unnamed method parameters.

## Flags/Config
```sh
-skip-single-param
    skip interfaces with a single unnamed parameter
```

## Usage 

### Standalone
You can run it standalone through `go vet`.  

You must install the binary to your `$GOBIN` folder like so:
```sh
$ go install github.com/macabu/inamedparam/cmd/inamedparam@latest
```

And then navigate to your Go project's root folder, where can run `go vet` in the following way:
```sh
$ go vet -vettool=$(which inamedparam) ./...
```

### golangci-lint
`inamedparam` was added as a linter to `golangci-lint` on version `v1.55.0`. It is disabled by default.

To enable it, you can add it to your `.golangci.yml` file, as such:
```yaml
run:
  timeout: 30s 

linters:
  disable-all: true
  enable:
    - inamedparam

linters-settings:
  inamedparam:
    # Skips check for interface methods with only a single parameter.
    # Default: false
    skip-single-param: true
```

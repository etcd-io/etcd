# cache

Extracted from `go/src/cmd/go/internal/cache/`.

The main modifications are:
- The errors management
  - Some methods return error.
  - Some errors are returned instead of being ignored.
- The name of the env vars:
  - `GOCACHE` -> `GOLANGCI_LINT_CACHE`
  - `GOCACHEPROG` -> `GOLANGCI_LINT_CACHEPROG` 

## History

- https://github.com/golangci/golangci-lint/pull/5576
  - sync go1.24.1
- https://github.com/golangci/golangci-lint/pull/5100
  - Move package from `internal/cache` to `internal/go/cache`
- https://github.com/golangci/golangci-lint/pull/5098
  - sync with go1.23.2
  - sync with go1.22.8
  - sync with go1.21.13
  - sync with go1.20.14
  - sync with go1.19.13
  - sync with go1.18.10
  - sync with go1.17.13
  - sync with go1.16.15
  - sync with go1.15.15
  - sync with go1.14.15

## Previous History

Based on the initial PR/commit the based in a mix between go1.12 and go1.13:
- cache.go (go1.13)
- cache_test.go (go1.12?)
- default.go (go1.12?)
- hash.go (go1.13 and go1.12 are identical)
- hash_test.go -> (go1.12?)

Adapted for golangci-lint:
- https://github.com/golangci/golangci-lint/pull/699: initial code (contains modifications of the files)
- https://github.com/golangci/golangci-lint/pull/779: just a nolint (`cache.go`)
- https://github.com/golangci/golangci-lint/pull/788: only directory permissions changes (0777 -> 0744) (`cache.go`, `cache_test.go`, `default.go`)
- https://github.com/golangci/golangci-lint/pull/808: mainly related to logs and errors (`cache.go`, `default.go`, `hash.go`, `hash_test.go`)
- https://github.com/golangci/golangci-lint/pull/1063: `ioutil` -> `robustio` (`cache.go`)
- https://github.com/golangci/golangci-lint/pull/1070: add `t.Parallel()` inside `cache_test.go`
- https://github.com/golangci/golangci-lint/pull/1162: errors inside `cache.go`
- https://github.com/golangci/golangci-lint/pull/2318: `ioutil` -> `os` (`cache.go`, `cache_test.go`, `default.go`, `hash_test.go`)
- https://github.com/golangci/golangci-lint/pull/2352: Go doc typos
- https://github.com/golangci/golangci-lint/pull/3012: errors inside `cache.go` (`cache.go`, `default.go`)
- https://github.com/golangci/golangci-lint/pull/3196: constant for `GOLANGCI_LINT_CACHE` (`cache.go`)
- https://github.com/golangci/golangci-lint/pull/3204: add this file and `%w` in `fmt.Errorf` (`cache.go`)
- https://github.com/golangci/golangci-lint/pull/3604: remove `github.com/pkg/errors` (`cache.go`)

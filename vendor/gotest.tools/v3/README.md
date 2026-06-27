# gotest.tools

A collection of packages to augment `testing` and support common patterns.

[![PkgGoDev](https://pkg.go.dev/badge/gotest.tools/v3?status.svg)](https://pkg.go.dev/gotest.tools/v3)
[![CircleCI](https://circleci.com/gh/gotestyourself/gotest.tools/tree/main.svg?style=shield)](https://circleci.com/gh/gotestyourself/gotest.tools/tree/main)
[![Go Reportcard](https://goreportcard.com/badge/gotest.tools/v3)](https://goreportcard.com/report/gotest.tools/v3)

## Usage

With Go modules enabled

```
$ go get gotest.tools/v3
```

```
import "gotest.tools/v3/assert"
```

## Packages

* [assert](http://pkg.go.dev/gotest.tools/v3/assert) -
  compare values and fail the test when a comparison fails
* [env](http://pkg.go.dev/gotest.tools/v3/env) -
  test code which uses environment variables
* [fs](http://pkg.go.dev/gotest.tools/v3/fs) -
  create temporary files and compare a filesystem tree to an expected value
* [golden](http://pkg.go.dev/gotest.tools/v3/golden) -
  compare large multi-line strings against values frozen in golden files
* [icmd](http://pkg.go.dev/gotest.tools/v3/icmd) -
  execute binaries and test the output
* [poll](http://pkg.go.dev/gotest.tools/v3/poll) -
  test asynchronous code by polling until a desired state is reached
* [skip](http://pkg.go.dev/gotest.tools/v3/skip) -
  skip a test and print the source code of the condition used to skip the test

## Related

* [gotest.tools/gotestsum](https://github.com/gotestyourself/gotestsum) -
  go test runner with custom output
* [go testing patterns](https://github.com/gotestyourself/gotest.tools/wiki/Go-Testing-Patterns) -
  zero-dependency patterns for organizing test cases
* [test doubles and patching](https://github.com/gotestyourself/gotest.tools/wiki/Test-Doubles-And-Patching) -
  zero-dependency test doubles (fakes, spies, stubs, and mocks) and monkey patching patterns

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

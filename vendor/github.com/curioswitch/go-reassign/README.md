# reassign

A linter that detects when reassigning a top-level variable in another package.

## Install

```bash
go install github.com/curioswitch/go-reassign
```

## Usage

```bash
reassign ./...
```

Change the pattern to match against variables being reassigned. By default, only `EOF` and `Err*` variables are checked.

```bash
reassign -pattern ".*" ./...
```

## Background

Package variables are commonly used to define sentinel errors which callers can use with `errors.Is` to determine the
type of a returned `error`. Some examples exist in the standard [os](https://pkg.go.dev/os#pkg-variables) library.

Unfortunately, as with any variable, these are mutable, and it is possible to write this very dangerous code.

```go
package main
import "io"
func bad() {
	// breaks file reading
	io.EOF = nil
}
```

This caused a new pattern for [constant errors](https://dave.cheney.net/2016/04/07/constant-errors)
to gain popularity, but they don't work well with improvements to the `errors` package in recent versions of Go and may
be considered to be non-idiomatic compared to normal `errors.New`. If we can catch reassignment of sentinel errors, we
gain much of the safety of constant errors.

This linter will catch reassignment of variables in other packages. By default it intends to apply to as many codebases
as possible and only checks a restricted set of variable names, `EOF` and `ErrFoo`, to restrict to sentinel errors. 
Package variable reassignment is generally confusing, though, and we recommend avoiding it for all variables, not just errors.
The `pattern` flag can be set to a regular expression to define what variables cannot be reassigned, and `.*` is 
recommended if it works with your code.

## Development

[mage](https://magefile.org/) is used for development. Run `go run mage.go -l` to see available targets.

For example, to run checks before sending a PR, run `go run mage.go check`.

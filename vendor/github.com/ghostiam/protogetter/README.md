# Protogetter
Welcome to the Protogetter project!

## Overview
Protogetter is a linter developed specifically for Go programmers working with nested `protobuf` types.\
It's designed to aid developers in preventing `invalid memory address or nil pointer dereference` errors arising from direct access of nested `protobuf` fields.

When working with `protobuf`, it's quite common to have complex structures where a message field is contained within another message, which itself can be part of another message, and so on.
If these fields are accessed directly and some field in the call chain will not be initialized, it can result in application panic.

Protogetter addresses this issue by suggesting use of getter methods for field access.

## How does it work?
Protogetter analyzes your Go code and helps detect direct `protobuf` field accesses that could give rise to panic.\
The linter suggests using getters:
```go
m.GetFoo().GetBar().GetBaz()
```
instead of direct field access:
```go
m.Foo.Bar.Baz
```

And you will then only need to perform a nil check after the final call:
```go
if m.GetFoo().GetBar().GetBaz() != nil {
    // Do something with m.GetFoo().GetBar().GetBaz()
}
```
instead of:
```go
if m.Foo != nil {
    if m.Foo.Bar != nil {
        if m.Foo.Bar.Baz != nil {
            // Do something with m.Foo.Bar.Baz
        }
    }
}
```

or use zero values:

```go
// If one of the methods returns `nil` we will receive 0 instead of panic.
v := m.GetFoo().GetBar().GetBaz().GetInt() 
```

instead of panic:

```go
// If at least one structure in the chains is not initialized, we will get a panic. 
v := m.Foo.Bar.Baz.Int
```

which simplifies the code and makes it more reliable.

## Usage

### Recommended way — via golangci-lint

Protogetter is integrated into [golangci-lint](https://github.com/golangci/golangci-lint) and can be run together with other linters. This is the preferred way to use it in most projects.

Example minimal `.golangci.yml` configuration:

```yaml
linters:
  enable:
    - protogetter
```

Run:

```bash
golangci-lint run ./...
```

## Standalone usage

### Installation

```bash
go install github.com/ghostiam/protogetter/cmd/protogetter@latest
```

### Direct run

To run the linter:
```bash
protogetter ./...
```

Or to apply suggested fixes directly:
```bash
protogetter --fix ./...
```

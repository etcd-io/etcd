**Note: This is a fork of the great project [go-sumtype](https://github.com/BurntSushi/go-sumtype) by BurntSushi.**
**The original seems largely unmaintained, and the changes in this fork are backwards incompatible.**

# go-check-sumtype [![CI](https://github.com/alecthomas/go-check-sumtype/actions/workflows/ci.yml/badge.svg)](https://github.com/alecthomas/go-check-sumtype/actions/workflows/ci.yml)
A simple utility for running exhaustiveness checks on type switch statements.
Exhaustiveness checks are only run on interfaces that are declared to be
"sum types."

Dual-licensed under MIT or the [UNLICENSE](http://unlicense.org).

This work was inspired by our code at
[Diffeo](https://diffeo.com).

## Installation

```go
$ go get github.com/alecthomas/go-check-sumtype
```

For usage info, just run the command:

```
$ go-check-sumtype
```

Typical usage might look like this:

```
$ go-check-sumtype $(go list ./... | grep -v vendor)
```

## Usage

`go-check-sumtype` takes a list of Go package paths or files and looks for sum type
declarations in each package/file provided. Exhaustiveness checks are then
performed for each use of a declared sum type in a type switch statement.
Namely, `go-check-sumtype` will report an error for any type switch statement that
either lacks a `default` clause or does not account for all possible variants.

Declarations are provided in comments like so:

```
//sumtype:decl
type MySumType interface { ... }
```

`MySumType` must be *sealed*. That is, part of its interface definition
contains an unexported method.

`go-check-sumtype` will produce an error if any of the above is not true.

For valid declarations, `go-check-sumtype` will look for all occurrences in which a
value of type `MySumType` participates in a type switch statement. In those
occurrences, it will attempt to detect whether the type switch is exhaustive
or not. If it's not, `go-check-sumtype` will report an error. For example, running
`go-check-sumtype` on this source file:

```go
package main

//sumtype:decl
type MySumType interface {
        sealed()
}

type VariantA struct{}

func (*VariantA) sealed() {}

type VariantB struct{}

func (*VariantB) sealed() {}

func main() {
        switch MySumType(nil).(type) {
        case *VariantA:
        }
}
```

produces the following:

```
$ sumtype mysumtype.go
mysumtype.go:18:2: exhaustiveness check failed for sum type 'MySumType': missing cases for VariantB
```

Adding either a `default` clause or a clause to handle `*VariantB` will cause
exhaustive checks to pass. To prevent `default` clauses from automatically
passing checks, set the `-default-signifies-exhasutive=false` flag.

As a special case, if the type switch statement contains a `default` clause
that always panics, then exhaustiveness checks are still performed.

By default, `go-check-sumtype` will not include shared interfaces in the exhaustiviness check.
This can be changed by setting the `-include-shared-interfaces=true` flag.
When this flag is set, `go-check-sumtype` will not require that all concrete structs
are listed in the switch statement, as long as the switch statement is exhaustive
with respect to interfaces the structs implement.

## Details and motivation

Sum types are otherwise known as discriminated unions. That is, a sum type is
a finite set of disjoint values. In type systems that support sum types, the
language will guarantee that if one has a sum type `T`, then its value must
be one of its variants.

Go's type system does not support sum types. A typical proxy for representing
sum types in Go is to use an interface with an unexported method and define
each variant of the sum type in the same package to satisfy said interface.
This guarantees that the set of types that satisfy the interface is closed
at compile time. Performing case analysis on these types is then done with
a type switch statement, e.g., `switch x.(type) { ... }`. Each clause of the
type switch corresponds to a *variant* of the sum type. The downside of this
approach is that Go's type system is not aware of the set of variants, so it
cannot tell you whether case analysis over a sum type is complete or not.

The `go-check-sumtype` command recognizes this pattern, but it needs a small amount
of help to recognize which interfaces should be treated as sum types, which
is why the `//sumtype:decl` annotation is required. `go-check-sumtype` will
figure out all of the variants of a sum type by finding the set of types
defined in the same package that satisfy the interface specified by the
declaration.

The `go-check-sumtype` command will prove its worth when you need to add a variant
to an existing sum type. Running `go-check-sumtype` will tell you immediately which
case analyses need to be updated to account for the new variant.

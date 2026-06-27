# astfmt

[![build-img]][build-url]
[![pkg-img]][pkg-url]
[![reportcard-img]][reportcard-url]
[![version-img]][version-url]

Package `astfmt` implements ast.Node formatting with fmt-like API.

## Installation

Go version 1.16+

```bash
go get github.com/go-toolsmith/astfmt
```

## Example

```go
package main

import (
	"go/token"
	"os"

	"github.com/go-toolsmith/astfmt"
	"github.com/go-toolsmith/strparse"
)

func Example() {
	x := strparse.Expr(`foo(bar(baz(1+2)))`)
	// astfmt functions add %s support for ast.Node arguments.
	astfmt.Println(x)                         // => foo(bar(baz(1 + 2)))
	astfmt.Fprintf(os.Stdout, "node=%s\n", x) // => node=foo(bar(baz(1 + 2)))

	// Can use specific file set with printer.
	fset := token.NewFileSet() // Suppose this fset is used when parsing
	pp := astfmt.NewPrinter(fset)
	pp.Println(x) // => foo(bar(baz(1 + 2)))
}
```

## License

[MIT License](LICENSE).

[build-img]: https://github.com/go-toolsmith/astfmt/workflows/build/badge.svg
[build-url]: https://github.com/go-toolsmith/astfmt/actions
[pkg-img]: https://pkg.go.dev/badge/go-toolsmith/astfmt
[pkg-url]: https://pkg.go.dev/github.com/go-toolsmith/astfmt
[reportcard-img]: https://goreportcard.com/badge/go-toolsmith/astfmt
[reportcard-url]: https://goreportcard.com/report/go-toolsmith/astfmt
[version-img]: https://img.shields.io/github/v/release/go-toolsmith/astfmt
[version-url]: https://github.com/go-toolsmith/astfmt/releases

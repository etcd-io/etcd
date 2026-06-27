# strparse

[![build-img]][build-url]
[![pkg-img]][pkg-url]
[![reportcard-img]][reportcard-url]
[![version-img]][version-url]

Package `strparse` provides convenience wrappers around `go/parser` for simple
expression, statement and declaretion parsing from string.

## Installation

Go version 1.16+

```bash
go get github.com/go-toolsmith/strparse
```

## Example

```go
package main

import (
	"github.com/go-toolsmith/astequal"
	"github.com/go-toolsmith/strparse"
)

func main() {
	// Comparing AST strings for equallity (note different spacing):
	x := strparse.Expr(`1 + f(v[0].X)`)
	y := strparse.Expr(` 1+f( v[0].X ) `)
	fmt.Println(astequal.Expr(x, y)) // => true
}
```

## License

[MIT License](LICENSE).

[build-img]: https://github.com/go-toolsmith/strparse/workflows/build/badge.svg
[build-url]: https://github.com/go-toolsmith/strparse/actions
[pkg-img]: https://pkg.go.dev/badge/go-toolsmith/strparse
[pkg-url]: https://pkg.go.dev/github.com/go-toolsmith/strparse
[reportcard-img]: https://goreportcard.com/badge/go-toolsmith/strparse
[reportcard-url]: https://goreportcard.com/report/go-toolsmith/strparse
[version-img]: https://img.shields.io/github/v/release/go-toolsmith/strparse
[version-url]: https://github.com/go-toolsmith/strparse/releases

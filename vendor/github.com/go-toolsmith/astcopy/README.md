# astcopy

[![build-img]][build-url]
[![pkg-img]][pkg-url]
[![reportcard-img]][reportcard-url]
[![version-img]][version-url]

Package `astcopy` implements Go AST reflection-free deep copy operations.

## Installation:

Go version 1.16+

```bash
go get github.com/go-toolsmith/astcopy
```

## Example

```go
package main

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/go-toolsmith/astcopy"
	"github.com/go-toolsmith/astequal"
	"github.com/go-toolsmith/strparse"
)

func main() {
	x := strparse.Expr(`1 + 2`).(*ast.BinaryExpr)
	y := astcopy.BinaryExpr(x)
	fmt.Println(astequal.Expr(x, y)) // => true

	// Now modify x and make sure y is not modified.
	z := astcopy.BinaryExpr(y)
	x.Op = token.SUB
	fmt.Println(astequal.Expr(y, z)) // => true
	fmt.Println(astequal.Expr(x, y)) // => false
}
```

## License

[MIT License](LICENSE).

[build-img]: https://github.com/go-toolsmith/astp/workflows/build/badge.svg
[build-url]: https://github.com/go-toolsmith/astp/actions
[pkg-img]: https://pkg.go.dev/badge/go-toolsmith/astp
[pkg-url]: https://pkg.go.dev/github.com/go-toolsmith/astp
[reportcard-img]: https://goreportcard.com/badge/go-toolsmith/astp
[reportcard-url]: https://goreportcard.com/report/go-toolsmith/astp
[version-img]: https://img.shields.io/github/v/release/go-toolsmith/astp
[version-url]: https://github.com/go-toolsmith/astp/releases

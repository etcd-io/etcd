# astp

[![build-img]][build-url]
[![pkg-img]][pkg-url]
[![reportcard-img]][reportcard-url]
[![version-img]][version-url]

Package `astp` provides AST predicates.

## Installation:

Go version 1.16+

```bash
go get github.com/go-toolsmith/astp
```

## Example

```go
package main

import (
	"fmt"

	"github.com/go-toolsmith/astp"
	"github.com/go-toolsmith/strparse"
)

func main() {
	if astp.IsIdent(strparse.Expr(`x`)) {
		fmt.Println("ident")
	}
	if astp.IsBlockStmt(strparse.Stmt(`{f()}`)) {
		fmt.Println("block stmt")
	}
	if astp.IsGenDecl(strparse.Decl(`var x int = 10`)) {
		fmt.Println("gen decl")
	}
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

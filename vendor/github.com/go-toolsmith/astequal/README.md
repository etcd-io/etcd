# astequal

[![build-img]][build-url]
[![pkg-img]][pkg-url]
[![reportcard-img]][reportcard-url]
[![version-img]][version-url]

Package `astequal` provides AST (deep) equallity check operations.

## Installation:

Go version 1.16+

```bash
go get github.com/go-toolsmith/astequal
```

## Example

```go
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"reflect"

	"github.com/go-toolsmith/astequal"
)

func main() {
	const code = `
		package foo

		func main() {
			x := []int{1, 2, 3}
			x := []int{1, 2, 3}
		}`

	fset := token.NewFileSet()
	pkg, err := parser.ParseFile(fset, "string", code, 0)
	if err != nil {
		log.Fatalf("parse error: %+v", err)
	}

	fn := pkg.Decls[0].(*ast.FuncDecl)
	x := fn.Body.List[0]
	y := fn.Body.List[1]

	// Reflect DeepEqual will fail due to different Pos values.
	// astequal only checks whether two nodes describe AST.
	fmt.Println(reflect.DeepEqual(x, y)) // => false
	fmt.Println(astequal.Node(x, y))     // => true
	fmt.Println(astequal.Stmt(x, y))     // => true
}
```

## Performance

`astequal` outperforms reflection-based comparison by a big margin:

```
BenchmarkEqualExpr/astequal.Expr-8       5000000     298 ns/op       0 B/op   0 allocs/op
BenchmarkEqualExpr/astequal.Node-8       3000000     409 ns/op       0 B/op   0 allocs/op
BenchmarkEqualExpr/reflect.DeepEqual-8     50000   38898 ns/op   10185 B/op   156 allocs/op
```

## License

[MIT License](LICENSE).

[build-img]: https://github.com/go-toolsmith/astequal/workflows/build/badge.svg
[build-url]: https://github.com/go-toolsmith/astequal/actions
[pkg-img]: https://pkg.go.dev/badge/go-toolsmith/astequal
[pkg-url]: https://pkg.go.dev/github.com/go-toolsmith/astequal
[reportcard-img]: https://goreportcard.com/badge/go-toolsmith/astequal
[reportcard-url]: https://goreportcard.com/report/go-toolsmith/astequal
[version-img]: https://img.shields.io/github/v/release/go-toolsmith/astequal
[version-url]: https://github.com/go-toolsmith/astequal/releases

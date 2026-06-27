# typep

[![build-img]][build-url]
[![pkg-img]][pkg-url]
[![reportcard-img]][reportcard-url]
[![version-img]][version-url]

Package `typep` provides type predicates.

## Installation:

Go version 1.16+

```bash
go get github.com/go-toolsmith/typep
```

## Example

```go
package main

import (
	"fmt"

	"github.com/go-toolsmith/typep"
	"github.com/go-toolsmith/strparse"
)

func main() {
	floatTyp := types.Typ[types.Float32]
	intTyp := types.Typ[types.Int]
	ptr := types.NewPointer(intTyp)
	arr := types.NewArray(intTyp, 64)

	fmt.Println(typep.HasFloatProp(floatTyp)) // => true
	fmt.Println(typep.HasFloatProp(intTyp))   // => false
	fmt.Println(typep.IsPointer(ptr))         // => true
	fmt.Println(typep.IsArray(arr))           // => true
}
```

## License

[MIT License](LICENSE).

[build-img]: https://github.com/go-toolsmith/typep/workflows/build/badge.svg
[build-url]: https://github.com/go-toolsmith/typep/actions
[pkg-img]: https://pkg.go.dev/badge/go-toolsmith/typep
[pkg-url]: https://pkg.go.dev/github.com/go-toolsmith/typep
[reportcard-img]: https://goreportcard.com/badge/go-toolsmith/typep
[reportcard-url]: https://goreportcard.com/report/go-toolsmith/typep
[version-img]: https://img.shields.io/github/v/release/go-toolsmith/typep
[version-url]: https://github.com/go-toolsmith/typep/releases

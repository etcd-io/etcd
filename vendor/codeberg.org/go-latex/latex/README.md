# latex

[![go.dev reference](https://pkg.go.dev/badge/codeberg.org/go-latex/latex)](https://pkg.go.dev/codeberg.org/go-latex/latex)
[![Codeberg Release](https://img.shields.io/gitea/v/release/go-latex/latex?gitea_url=https%3A%2F%2Fcodeberg.org)](https://codeberg.org/go-latex/latex/releases)
[![CI](https://codeberg.org/go-latex/latex/workflows/CI/badge.svg)](https://codeberg.org/go-latex/latex/actions)
[![codecov](https://codecov.io/gh/go-latex/latex/branch/main/graph/badge.svg)](https://codecov.io/gh/go-latex/latex)
[![GoDoc](https://godoc.org/codeberg.org/go-latex/latex?status.svg)](https://godoc.org/codeberg.org/go-latex/latex)
[![License](https://img.shields.io/badge/License-BSD--3-blue.svg)](https://codeberg.org/go-latex/latex/raw/main/LICENSE)

`latex` is a package holding Go tools for [LaTeX](https://www.latex-project.org/).

`latex` is supposed to provide features akin to `MathJax` or `matplotlib`'s `TeX` capabilities.
_ie:_ it is supposed to be able to draw mathematical equations, in pure-Go.

`latex` is *NOT SUPPOSED* to be a complete typesetting system like `LaTeX` or `TeX`.

For this, please take look at:

- [Star-TeX](https://star-tex.org)
- [Star-TeX (git repo)](https://git.sr.ht/~sbinet/star-tex)

Eventually, `go-latex/latex` might just use `star-tex.org/...` to provide the `MathJax`-like capabilities.
(once `star-tex.org/...` is ready and exports a nice Go API.)

## Installation

```
$> go get codeberg.org/go-latex/latex/...
```

## Documentation

Documentation is served by [godoc](https://godoc.org), here:

- [godoc.org/codeberg.org/go-latex/latex](https://godoc.org/codeberg.org/go-latex/latex)

The main use case for `go-latex/latex` is to draw a mathematical equation.
This is typically achieved via the `latex/mtex.Render` function that knows how to render mathematical `TeX` equations to a renderer interface.

### Example

```go
package main

import (
	"os"

	"codeberg.org/go-latex/latex/drawtex/drawimg"
	"codeberg.org/go-latex/latex/mtex"
)

func main() {
	f, err := os.Create("output.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	dst := drawimg.NewRenderer(f)
	err = mtex.Render(dst, `$f(x) = \frac{\sqrt{x +20}}{2\pi} +\hbar \sum y\partial y$`, 12, 72, nil)
	if err != nil {
		panic(err)
	}

	err = f.Close()
	if err != nil {
		panic(err)
	}
}
```

## LICENSE

BSD-3.

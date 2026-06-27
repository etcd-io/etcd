# exhaustive

[![Godoc][godoc-svg]][godoc]

`exhaustive` checks exhaustiveness of enum switch statements in Go source code.

For the definition of enum and the definition of exhaustiveness used by this
program, see [godoc][godoc-doc]. For the changelog, see [CHANGELOG][changelog]
in the GitHub wiki. The program can be configured to additionally check
exhaustiveness of keys in map literals whose key type is an enum.

## Usage

Command:

```
go install github.com/nishanths/exhaustive/cmd/exhaustive@latest

exhaustive [flags] [packages]
```

For available flags, refer to the [Flags][godoc-flags] section in godoc or run
`exhaustive -h`.

Package:

```
go get github.com/nishanths/exhaustive

import "github.com/nishanths/exhaustive"
```

The `exhaustive.Analyzer` variable follows guidelines in the
[`golang.org/x/tools/go/analysis`][xanalysis] package. This should make it
possible to integrate `exhaustive` with your own analysis driver program.

## Example

Given an enum:

```
package token // import "example.org/token"

type Token int

const (
	Add Token = iota
	Subtract
	Multiply
	Quotient
	Remainder
)
```

and code that switches on the enum:

```
package calc

import "example.org/token"

func x(t token.Token) {
	switch t {
	case token.Add:
	case token.Subtract:
	case token.Remainder:
	default:
	}
}
```

running `exhaustive` with default flags will produce:

```
calc.go:6:2: missing cases in switch of type token.Token: token.Multiply, token.Quotient
```

Specify flag `-check=switch,map` to additionally check exhaustiveness of keys
in map literals. For example:

```
var m = map[token.Token]rune{
	token.Add:      '+',
	token.Subtract: '-',
	token.Multiply: '*',
	token.Quotient: '/',
}
```

```
calc.go:14:9: missing keys in map of key type token.Token: token.Remainder
```

## Contributing

Issues and changes are welcome. Please discuss substantial changes in an issue
first.

[godoc]: https://pkg.go.dev/github.com/nishanths/exhaustive
[godoc-svg]: https://pkg.go.dev/badge/github.com/nishanths/exhaustive.svg
[godoc-doc]: https://pkg.go.dev/github.com/nishanths/exhaustive#section-documentation
[godoc-flags]: https://pkg.go.dev/github.com/nishanths/exhaustive#hdr-Flags
[xanalysis]: https://pkg.go.dev/golang.org/x/tools/go/analysis
[changelog]: https://github.com/nishanths/exhaustive/wiki/CHANGELOG

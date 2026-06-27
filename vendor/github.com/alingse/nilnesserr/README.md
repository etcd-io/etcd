# nilnesserr

nilnesserr = nilness + nilerr

`nilnesserr` is a linter for report return nil error in Go. It combines the features of [nilness](https://cs.opensource.google/go/x/tools/+/refs/tags/v0.28.0:go/analysis/passes/nilness/nilness.go) and [nilerr](https://github.com/gostaticanalysis/nilerr), providing a concise way to detect return an unrelated/nil-values error.

## Case

case 1
```go
err := do()
if err != nil {
    return err
}
err2 := do2()
if err2 != nil {
    return err // want `return a nil value error after check error`
}
```

case 2

```go
err := do()
if err != nil {
    return err
}
_, err2 := do2()
if err2 != nil {
    return errors.Wrap(err) // want `call function with a nil value error after check error`
}
```

case 3

```go
err := do()
if err != nil {
    return err
}
_, err2 := do2()
if err2 != nil {
    return fmt.Errorf("call do2 failed: %w",err) // want `call variadic function with a nil value error after check error`
}
```


## Some Real Bugs

- https://github.com/alingse/sundrylint/issues/4
- https://github.com/alingse/nilnesserr/issues/1
- https://github.com/alingse/nilnesserr/issues/11
- https://github.com/alingse/nilnesserr/issues/15

We use https://github.com/alingse/go-linter-runner to run linter on GitHub Actions for public Go repos

## Install

```bash
go install github.com/alingse/nilnesserr/cmd/nilnesserr@latest
```


## TODO

case 3

```go
err := do()
if err != nil {
    return err
}
_, ok := do2()
if !ok {
    return err
}

```

maybe this is also a bug, should return a non-nil value error after the if

## License

This project is licensed under the MIT License. See the LICENSE file for details.

This project incorporates source code from two different libraries:

1. [nilness](https://cs.opensource.google/go/x/tools/+/refs/tags/v0.28.0:go/analysis/passes/nilness/nilness.go) licensed under the BSD license.
2. [nilerr](https://github.com/gostaticanalysis/nilerr) licensed under the MIT license.

# Go Tag Align

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/4meepo/tagalign?style=flat-square)
[![codecov](https://codecov.io/github/4meepo/tagalign/branch/main/graph/badge.svg?token=1R1T61UNBQ)](https://codecov.io/github/4meepo/tagalign)
[![GoDoc](https://godoc.org/github.com/4meepo/tagalign?status.svg)](https://pkg.go.dev/github.com/4meepo/tagalign)
[![Go Report Card](https://goreportcard.com/badge/github.com/4meepo/tagalign)](https://goreportcard.com/report/github.com/4meepo/tagalign)

TagAlign is used to align and sort tags in Go struct. It can make the struct more readable and easier to maintain.

For example, this struct

```go
type FooBar struct {
    Foo    int    `json:"foo" validate:"required"`
    Bar    string `json:"bar" validate:"required"`
    FooFoo int8   `json:"foo_foo" validate:"required"`
    BarBar int    `json:"bar_bar" validate:"required"`
    FooBar struct {
    Foo    int    `json:"foo" yaml:"foo" validate:"required"`
    Bar222 string `json:"bar222" validate:"required" yaml:"bar"`
    } `json:"foo_bar" validate:"required"`
    BarFoo    string `json:"bar_foo" validate:"required"`
    BarFooBar string `json:"bar_foo_bar" validate:"required"`
}
```

can be aligned to:

```go
type FooBar struct {
    Foo    int    `json:"foo"     validate:"required"`
    Bar    string `json:"bar"     validate:"required"`
    FooFoo int8   `json:"foo_foo" validate:"required"`
    BarBar int    `json:"bar_bar" validate:"required"`
    FooBar struct {
        Foo    int    `json:"foo"    yaml:"foo"          validate:"required"`
        Bar222 string `json:"bar222" validate:"required" yaml:"bar"`
    } `json:"foo_bar" validate:"required"`
    BarFoo    string `json:"bar_foo"     validate:"required"`
    BarFooBar string `json:"bar_foo_bar" validate:"required"`
}
```

## Usage

By default tagalign will only align tags, but not sort them. But alignment and [sort feature](https://github.com/4meepo/tagalign#sort-tag) can work together or separately.

* As a Golangci Linter (Recommended)

    Tagalign is a built-in linter in [Golangci Lint](https://golangci-lint.run/usage/linters/#tagalign) since `v1.53`.
    > Note: In order to have the best experience,  add the `--fix` flag to `golangci-lint` to enable the autofix feature.

* Standalone Mode

    Install it using `GO` or download it [here](https://github.com/4meepo/tagalign/releases).

    ```bash
    go install github.com/4meepo/tagalign/cmd/tagalign@latest
    ```

    Run it in your terminal.

    ```bash
    # Only align tags.
    tagalign -fix {package path}
    # Only sort tags with fixed order.
    tagalign -fix -noalign -sort -order "json,xml" {package path}
    # Align and sort together.
    tagalign -fix -sort -order "json,xml" {package path}
    # Align and sort together in strict style.
    tagalign -fix -sort -order "json,xml" -strict {package path}
    ```

## Advanced Features

### Sort Tag

In addition to alignment, it can also sort tags with fixed order. If we enable sort with fixed order `json,xml`, the following code

```go
type SortExample struct {
    Foo    int `json:"foo,omitempty" yaml:"bar" xml:"baz" binding:"required" gorm:"column:foo" zip:"foo" validate:"required"`
    Bar    int `validate:"required"  yaml:"foo" xml:"bar" binding:"required" json:"bar,omitempty" gorm:"column:bar" zip:"bar" `
    FooBar int `gorm:"column:bar" validate:"required"   xml:"bar" binding:"required" json:"bar,omitempty"  zip:"bar" yaml:"foo"`
}
```

will be sorted and aligned to:

```go
type SortExample struct {
    Foo    int `json:"foo,omitempty" xml:"baz" binding:"required" gorm:"column:foo" validate:"required" yaml:"bar" zip:"foo"`
    Bar    int `json:"bar,omitempty" xml:"bar" binding:"required" gorm:"column:bar" validate:"required" yaml:"foo" zip:"bar"`
    FooBar int `json:"bar,omitempty" xml:"bar" binding:"required" gorm:"column:bar" validate:"required" yaml:"foo" zip:"bar"`
}
```

The fixed order is `json,xml`, so the tags `json` and `xml` will be sorted and aligned first, and the rest tags will be sorted and aligned in the dictionary order.

### Strict Style

Sometimes, you may want to align your tags in strict style. In this style, the tags will be sorted and aligned in the dictionary order, and the tags with the same name will be aligned together. For example, the following code

```go
type StrictStyleExample struct {
    Foo int ` xml:"baz" yaml:"bar" zip:"foo" binding:"required" gorm:"column:foo"  validate:"required"`
    Bar int `validate:"required" gorm:"column:bar"  yaml:"foo" xml:"bar" binding:"required" json:"bar,omitempty" `
}
```

will be aligned to

```go
type StrictStyleExample struct {
    Foo int `binding:"required" gorm:"column:foo"                      validate:"required" xml:"baz" yaml:"bar" zip:"foo"`
    Bar int `binding:"required" gorm:"column:bar" json:"bar,omitempty" validate:"required" xml:"bar" yaml:"foo"`
}
```

> ⚠️Note: The strict style can't run without the align or sort feature enabled.

## References

[Golang AST Visualizer](http://goast.yuroyoro.net/)

[Create New Golang CI Linter](https://golangci-lint.run/contributing/new-linters/)

[Autofix Example](https://github.com/golangci/golangci-lint/pull/2450/files)

[Integrating](https://disaev.me/p/writing-useful-go-analysis-linter/#integrating)

# musttag

[![checks](https://github.com/go-simpler/musttag/actions/workflows/checks.yml/badge.svg)](https://github.com/go-simpler/musttag/actions/workflows/checks.yml)
[![pkg.go.dev](https://pkg.go.dev/badge/go-simpler.org/musttag.svg)](https://pkg.go.dev/go-simpler.org/musttag)
[![goreportcard](https://goreportcard.com/badge/go-simpler.org/musttag)](https://goreportcard.com/report/go-simpler.org/musttag)
[![codecov](https://codecov.io/gh/go-simpler/musttag/branch/main/graph/badge.svg)](https://codecov.io/gh/go-simpler/musttag)

A Go linter that enforces field tags in (un)marshaled structs.

## 📌 About

`musttag` checks that exported fields of a struct passed to a `Marshal`-like function are annotated with the relevant tag:

```go
// BAD:
var user struct {
    Name string
}
data, err := json.Marshal(user)

// GOOD:
var user struct {
    Name string `json:"name"`
}
data, err := json.Marshal(user)
```

The rational from [Uber Style Guide][1]:

> The serialized form of the structure is a contract between different systems.
> Changes to the structure of the serialized form, including field names, break this contract.
> Specifying field names inside tags makes the contract explicit,
> and it guards against accidentally breaking the contract by refactoring or renaming fields.

## 🚀 Features

The following packages are supported out of the box:

* [encoding/json][2]
* [encoding/xml][3]
* [gopkg.in/yaml.v3][4]
* [github.com/BurntSushi/toml][5]
* [github.com/mitchellh/mapstructure][6]
* [github.com/jmoiron/sqlx][7]

In addition, any [custom package](#custom-packages) can be added to the list.

## 📦 Install

`musttag` is integrated into [`golangci-lint`][8], and this is the recommended way to use it.

To enable the linter, add the following lines to `.golangci.yml`:

```yaml
linters:
  enable:
    - musttag
```

Alternatively, you can download a prebuilt binary from the [Releases][9] page to use `musttag` standalone.

## 📋 Usage

Run `golangci-lint` with `musttag` enabled.
See the list of [available options][10] to configure the linter.

When using `musttag` standalone, pass the options as flags.

### Custom packages

To report a custom function, you need to add its description to `.golangci.yml`.
The following is an example of adding support for [`hclsimple.Decode`][11] and [`validator.Validate.Struct`][12]:

```yaml
linters-settings:
  musttag:
    functions:
        # The full name of the function, including the package.
      - name: github.com/hashicorp/hcl/v2/hclsimple.Decode
        # The struct tag whose presence should be ensured.
        tag: hcl
        # The position of the argument to check.
        arg-pos: 2

        # If the function is a pointer receiver method, use this format:
      - name: (*github.com/go-playground/validator/v10.Validate).Struct
        tag: validate
        arg-pos: 0
```

The same can be done via the `-fn=<name:tag:arg-pos>` flag when using `musttag` standalone:

```shell
musttag -fn="github.com/hashicorp/hcl/v2/hclsimple.DecodeFile:hcl:2" -fn="(*github.com/go-playground/validator/v10.Validate).Struct:validate:0" ./...
```

[1]: https://github.com/uber-go/guide/blob/master/style.md#use-field-tags-in-marshaled-structs
[2]: https://pkg.go.dev/encoding/json
[3]: https://pkg.go.dev/encoding/xml
[4]: https://pkg.go.dev/gopkg.in/yaml.v3
[5]: https://pkg.go.dev/github.com/BurntSushi/toml
[6]: https://pkg.go.dev/github.com/mitchellh/mapstructure
[7]: https://pkg.go.dev/github.com/jmoiron/sqlx
[8]: https://golangci-lint.run
[9]: https://github.com/go-simpler/musttag/releases
[10]: https://golangci-lint.run/usage/linters/#musttag
[11]: https://pkg.go.dev/github.com/hashicorp/hcl/v2/hclsimple#Decode
[12]: https://pkg.go.dev/github.com/go-playground/validator/v10#Validate.Struct

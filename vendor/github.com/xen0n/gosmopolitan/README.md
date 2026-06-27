# gosmopolitan

![GitHub Workflow Status (main branch)](https://img.shields.io/github/actions/workflow/status/xen0n/gosmopolitan/go.yml?branch=main)
![Codecov](https://img.shields.io/codecov/c/gh/xen0n/gosmopolitan)
![GitHub license info](https://img.shields.io/github/license/xen0n/gosmopolitan)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/xen0n/gosmopolitan)
[![Go Report Card](https://goreportcard.com/badge/github.com/xen0n/gosmopolitan)](https://goreportcard.com/report/github.com/xen0n/gosmopolitan)
[![Go Reference](https://pkg.go.dev/badge/github.com/xen0n/gosmopolitan.svg)](https://pkg.go.dev/github.com/xen0n/gosmopolitan)

[简体中文](./README.zh-Hans.md)

`gosmopolitan` checks your Go codebase for code smells that may prove to be
hindrance to internationalization ("i18n") and/or localization ("l10n").

The name is a wordplay on "cosmopolitan".

## Checks

Currently `gosmopolitan` checks for the following anti-patterns:

*   Occurrences of string literals containing characters from certain writing
    systems.

    Existence of such strings often means the relevant logic is hard to
    internationalize, or at least, require special care when doing i18n/l10n.

*   Usages of `time.Local`.

    An internationalized app or library should almost never process time and
    date values in the timezone in which it is running; instead one should use
    the respective user preference, or the timezone as dictated by the domain
    logic.

Note that local times are produced in a lot more ways than via direct casts to
`time.Local` alone, such as:

* `time.LoadLocation("Local")`
* received from a `time.Ticker`
* functions explicitly documented to return local times
    * `time.Now()`
    * `time.Unix()`
    * `time.UnixMilli()`
    * `time.UnixMicro()`

Proper identification of these use cases require a fairly complete dataflow
analysis pass, which is not implemented currently. In addition, right now you
have to pay close attention to externally-provided time values (such as from
your framework like Gin or gRPC) as they are not properly tracked either.

## Caveats

Note that the checks implemented here are only suitable for codebases with the
following characteristics, and may not suit your particular project's needs:

* Originally developed for an audience using non-Latin writing system(s),
* Returns bare strings intended for humans containing such non-Latin characters, and
* May occasionally (or frequently) refer to the system timezone, but is
  architecturally forbidden/discouraged to just treat the system timezone as
  the reference timezone.

For example, the lints may prove valuable if you're revamping a web service
originally targetting the Chinese market (hence producing strings with Chinese
characters all over the place) to be more i18n-aware. Conversely, if you want
to identify some of the i18n-naïve places in an English-only app, the linter
will output nothing.

## golangci-lint integration

`gosmopolitan` support [has been merged][gcl-pr] into [`golangci-lint`][gcl-home],
and will be usable out-of-the-box in golangci-lint v1.53.0 or later.

Due to the opinionated coding style this linter advocates and checks for, if
you have `enable-all: true` in your `golangci.yml` and your project deals a
lot with Chinese text and/or `time.Local`, then you'll get flooded with lints
when you upgrade to golangci-lint v1.53.0. Just disable this linter (and
better yet, move away from `enable-all: true`) if the style does not suit your
specific use case.

[gcl-pr]: https://github.com/golangci/golangci-lint/pull/3458
[gcl-home]: https://golangci-lint.run

## License

`gosmopolitan` is licensed under the GPL license, version 3 or later.

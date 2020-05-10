# fixplurals [![Build Status](https://travis-ci.org/davidrjenni/reftools.svg?branch=master)](https://travis-ci.org/davidrjenni/reftools) [![Coverage Status](https://coveralls.io/repos/github/davidrjenni/reftools/badge.svg)](https://coveralls.io/github/davidrjenni/reftools) [![GoDoc](https://godoc.org/github.com/davidrjenni/reftools?status.svg)](https://godoc.org/github.com/davidrjenni/reftools/cmd/fixplurals) [![Go Report Card](https://goreportcard.com/badge/github.com/davidrjenni/reftools)](https://goreportcard.com/report/github.com/davidrjenni/reftools)

fixplurals - remove redundant parameter and result types from function signatures

---

For example, the following function signature:
```
func fun(a string, b string) (c string, d string)
```
becomes:
```
func fun(a, b string) (c, d string)
```
after applying fixplurals.

## Installation

```
% go get -u github.com/davidrjenni/reftools/cmd/fixplurals
```

## Usage

```
% fixplurals [-dry] packages
```

Flags:

	-dry: changes are printed to stdout instead of rewriting the source files

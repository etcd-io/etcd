# canonicalheader

[![CI](https://github.com/lasiar/canonicalheader/actions/workflows/test.yml/badge.svg)](https://github.com/lasiar/canonicalheader/actions/workflows/test.yml)
[![tag](https://img.shields.io/github/tag/lasiar/canonicalheader.svg)](https://github.com/lasiar/canonicalheader/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/lasiar/canonicalheader)](https://goreportcard.com/report/github.com/lasiar/canonicalheader)
[![License](https://img.shields.io/github/license/lasiar/canonicalheader)](./LICENCE)

Golang linter for check canonical header.

### Install

```shell
go install -v github.com/lasiar/canonicalheader/cmd/canonicalheader@latest 
```

Or download the binary file from the [release](https://github.com/lasiar/canonicalheader/releases/latest).


### Example

before

```go
package main

import (
	"net/http"
)

const testHeader = "testHeader"

func main() {
	v := http.Header{}
	v.Get(testHeader)

	v.Get("Test-HEader")
	v.Set("Test-HEader", "value")
	v.Add("Test-HEader", "value")
	v.Del("Test-HEader")
	v.Values("Test-HEader")

	v.Set("Test-Header", "value")
	v.Add("Test-Header", "value")
	v.Del("Test-Header")
	v.Values("Test-Header")
}

```

after

```go
package main

import (
	"net/http"
)

const testHeader = "testHeader"

func main() {
	v := http.Header{}
	v.Get(testHeader)

	v.Get("Test-Header")
	v.Set("Test-Header", "value")
	v.Add("Test-Header", "value")
	v.Del("Test-Header")
	v.Values("Test-Header")

	v.Set("Test-Header", "value")
	v.Add("Test-Header", "value")
	v.Del("Test-Header")
	v.Values("Test-Header")
}

```

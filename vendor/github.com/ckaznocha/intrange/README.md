# intrange

[![Build Status](https://github.com/ckaznocha/intrange/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/ckaznocha/intrange/actions/workflows/ci.yml)
[![Release](http://img.shields.io/github/release/ckaznocha/intrange.svg)](https://github.com/ckaznocha/intrange/releases/latest)
[![GoDoc](https://godoc.org/github.com/ckaznocha/intrange?status.svg)](https://godoc.org/github.com/ckaznocha/intrange)

intrange is a program for checking for loops that could use the [Go 1.22](https://go.dev/ref/spec#Go_1.22) integer
range feature.

## Installation

```bash
go install github.com/ckaznocha/intrange/cmd/intrange@latest
```

## Usage

```bash
go vet -vettool=$(which intrange) ./...
```

## Examples

### A loop that uses the value of the loop variable

```go
package main

import "fmt"

func main() {
    for i := 0; i < 10; i++ {
        fmt.Println(i)
    }
}
```

Running `intrange` on the above code will produce the following output:

```bash
main.go:5:2: for loop can be changed to use an integer range (Go 1.22+)
```

The loop can be rewritten as:

```go
package main

import "fmt"

func main() {
    for i := range 10 {
        fmt.Println(i)
    }
}
```

### A loop that does not use the value of the loop variable

```go
package main

import "fmt"

func main() {
    for i := 0; i < 10; i++ {
        fmt.Println("Hello again!")
    }
}
```

Running `intrange` on the above code will produce the following output:

```bash
main.go:5:2: for loop can be changed to use an integer range (Go 1.22+)
```

The loop can be rewritten as:

```go
package main

import "fmt"

func main() {
    for range 10 {
        fmt.Println("Hello again!")
    }
}
```
